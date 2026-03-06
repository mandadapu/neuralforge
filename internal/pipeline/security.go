package pipeline

import (
	"context"
	"fmt"

	"github.com/mandadapu/neuralforge/internal/llm"
)

type SecurityStage struct {
	llm llm.LLM
}

func NewSecurityStage(l llm.LLM) *SecurityStage {
	return &SecurityStage{llm: l}
}

func (s *SecurityStage) Name() string { return "security" }

func (s *SecurityStage) Run(ctx context.Context, state *PipelineState) (StageResult, error) {
	if state.Plan == "" {
		return StageResult{
			Status: StatusSkipped,
			Output: "no plan to review",
		}, nil
	}

	prompt := fmt.Sprintf(
		"You are a security engineer. Review the following implementation plan "+
			"for potential security risks, vulnerabilities, and best-practice violations.\n\n"+
			"Repository: %s\n\n"+
			"Plan:\n%s\n\n"+
			"List any security concerns, rated by severity (critical, high, medium, low). "+
			"If the plan is safe, say so explicitly.",
		state.Repo.FullName, state.Plan,
	)

	resp, err := s.llm.Complete(ctx, llm.CompletionRequest{
		System: "You are a security reviewer analyzing implementation plans for vulnerabilities.",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: prompt},
		},
		MaxTokens:   2048,
		Temperature: 0.2,
	})
	if err != nil {
		return StageResult{}, fmt.Errorf("security llm call: %w", err)
	}

	state.SecurityNotes = resp.Content
	state.Cost += resp.Cost

	return StageResult{
		Status: StatusPassed,
		Output: fmt.Sprintf("Security review complete (%d chars)", len(resp.Content)),
	}, nil
}
