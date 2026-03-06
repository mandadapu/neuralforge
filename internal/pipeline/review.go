package pipeline

import (
	"context"
	"fmt"
	"strings"

	"github.com/mandadapu/neuralforge/internal/llm"
)

type ReviewStage struct {
	llm    llm.LLM
	client GitHubClient
}

func NewReviewStage(l llm.LLM, client GitHubClient) *ReviewStage {
	return &ReviewStage{llm: l, client: client}
}

func (s *ReviewStage) Name() string { return "review" }

func (s *ReviewStage) Run(ctx context.Context, state *PipelineState) (StageResult, error) {
	if state.PRNumber == 0 {
		return StageResult{
			Status: StatusSkipped,
			Output: "No PR to review",
		}, nil
	}

	// Build a summary of changed files.
	var changedFiles strings.Builder
	for _, c := range state.Changes {
		fmt.Fprintf(&changedFiles, "- %s (%s)\n%s\n\n", c.Path, c.Action, c.Diff)
	}

	testOutput := ""
	if state.TestResults != nil {
		testOutput = state.TestResults.Output
	}

	prompt := fmt.Sprintf(
		"You are a senior code reviewer. Review the following changes made to fix GitHub issue #%d: %s\n\n"+
			"## Plan\n%s\n\n"+
			"## Files Changed\n%s\n\n"+
			"## Test Results\n%s\n\n"+
			"Provide a concise review. If there are critical issues, start your response with REQUEST_CHANGES. "+
			"If the changes look good, start your response with APPROVE. "+
			"Then explain your reasoning.",
		state.Issue.Number, state.Issue.Title,
		state.Plan,
		changedFiles.String(),
		testOutput,
	)

	resp, err := s.llm.Complete(ctx, llm.CompletionRequest{
		System: "You are a code reviewer. Start your response with APPROVE or REQUEST_CHANGES.",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   2048,
		Temperature: 0.3,
	})
	if err != nil {
		return StageResult{}, fmt.Errorf("review llm call: %w", err)
	}

	state.ReviewNotes = resp.Content
	state.Cost += resp.Cost

	// Determine review event based on LLM output.
	parts := strings.SplitN(state.Repo.FullName, "/", 2)
	if len(parts) != 2 {
		return StageResult{}, fmt.Errorf("invalid repo name: %s", state.Repo.FullName)
	}
	owner, repo := parts[0], parts[1]

	event := "APPROVE"
	if strings.HasPrefix(strings.TrimSpace(resp.Content), "REQUEST_CHANGES") {
		event = "REQUEST_CHANGES"
	}

	if err := s.client.CreateReview(ctx, owner, repo, state.PRNumber, resp.Content, event); err != nil {
		return StageResult{}, fmt.Errorf("post review: %w", err)
	}

	status := StatusPassed
	if event == "REQUEST_CHANGES" {
		status = StatusFailed
	}

	return StageResult{
		Status: status,
		Output: fmt.Sprintf("Review posted: %s", event),
	}, nil
}
