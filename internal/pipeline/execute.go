package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/mandadapu/neuralforge/internal/executor"
)

type ExecuteStage struct {
	exec    executor.Executor
	timeout time.Duration
}

func NewExecuteStage(exec executor.Executor, timeout time.Duration) *ExecuteStage {
	return &ExecuteStage{exec: exec, timeout: timeout}
}

func (s *ExecuteStage) Name() string { return "execute" }

func (s *ExecuteStage) Run(ctx context.Context, state *PipelineState) (StageResult, error) {
	branch := fmt.Sprintf("neuralforge/issue-%d", state.Issue.Number)

	job := executor.ExecutorJob{
		ID:       state.JobID,
		RepoPath: state.Repo.LocalPath,
		Branch:   branch,
		Prompt:   state.Plan,
		Context:  state.Memory,
		Timeout:  s.timeout,
	}

	result, err := s.exec.Run(ctx, job)
	if err != nil {
		return StageResult{}, fmt.Errorf("executor run: %w", err)
	}

	if result.TimedOut {
		return StageResult{
			Status: StatusFailed,
			Output: "execution timed out",
		}, nil
	}

	if !result.Success {
		return StageResult{
			Status: StatusFailed,
			Output: fmt.Sprintf("execution failed: %s", result.Stderr),
		}, nil
	}

	changes := make([]FileChange, len(result.FilesChanged))
	for i, f := range result.FilesChanged {
		changes[i] = FileChange{
			Path:   f,
			Action: "modified",
		}
	}
	state.Changes = changes

	return StageResult{
		Status: StatusPassed,
		Output: fmt.Sprintf("Executed successfully, %d files changed", len(result.FilesChanged)),
	}, nil
}
