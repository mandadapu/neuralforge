package pipeline

import (
	"context"
	"fmt"

	"github.com/mandadapu/neuralforge/internal/llm"
)

type ArchitectStage struct {
	llm llm.LLM
}

func NewArchitectStage(l llm.LLM) *ArchitectStage {
	return &ArchitectStage{llm: l}
}

func (s *ArchitectStage) Name() string { return "architect" }

func (s *ArchitectStage) Run(ctx context.Context, state *PipelineState) (StageResult, error) {
	prompt := fmt.Sprintf(
		"You are a senior software architect. Given the following GitHub issue, "+
			"create a detailed implementation plan.\n\n"+
			"Issue #%d: %s\n\n%s\n\n"+
			"Codebase context:\n%s\n\n"+
			"Provide a step-by-step implementation plan including files to modify, "+
			"functions to add or change, and any dependencies.",
		state.Issue.Number, state.Issue.Title, state.Issue.Body, state.Memory,
	)

	resp, err := s.llm.Complete(ctx, llm.CompletionRequest{
		System: "You are a software architect creating implementation plans.",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   4096,
		Temperature: 0.3,
	})
	if err != nil {
		return StageResult{}, fmt.Errorf("architect llm call: %w", err)
	}

	state.Plan = resp.Content
	state.Cost += resp.Cost

	return StageResult{
		Status: StatusPassed,
		Output: fmt.Sprintf("Generated implementation plan (%d chars)", len(resp.Content)),
	}, nil
}
