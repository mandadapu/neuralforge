package pipeline

import (
	"context"
	"fmt"
	"strings"
)

type MergeStage struct {
	client    GitHubClient
	autoMerge bool
}

func NewMergeStage(client GitHubClient, autoMerge bool) *MergeStage {
	return &MergeStage{client: client, autoMerge: autoMerge}
}

func (s *MergeStage) Name() string { return "merge" }

func (s *MergeStage) Run(ctx context.Context, state *PipelineState) (StageResult, error) {
	if !s.autoMerge {
		return StageResult{
			Status: StatusSkipped,
			Output: "Auto-merge disabled",
		}, nil
	}

	if state.PRNumber == 0 {
		return StageResult{
			Status: StatusSkipped,
			Output: "No PR to merge",
		}, nil
	}

	if strings.Contains(state.ReviewNotes, "REQUEST_CHANGES") {
		return StageResult{
			Status: StatusFailed,
			Output: "Review requested changes — merge blocked",
		}, nil
	}

	parts := strings.SplitN(state.Repo.FullName, "/", 2)
	if len(parts) != 2 {
		return StageResult{}, fmt.Errorf("invalid repo name: %s", state.Repo.FullName)
	}
	owner, repo := parts[0], parts[1]

	message := fmt.Sprintf("fix: #%d (auto-merged by NeuralForge)", state.Issue.Number)
	if err := s.client.MergePR(ctx, owner, repo, state.PRNumber, message); err != nil {
		return StageResult{}, fmt.Errorf("merge PR: %w", err)
	}

	state.Merged = true

	return StageResult{
		Status: StatusPassed,
		Output: fmt.Sprintf("PR #%d merged", state.PRNumber),
	}, nil
}
