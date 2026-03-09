package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/mandadapu/neuralforge/internal/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteStage_Name(t *testing.T) {
	s := NewExecuteStage(&mockExecutor{}, time.Minute)
	assert.Equal(t, "execute", s.Name())
}

func TestExecuteStage_Success(t *testing.T) {
	exec := &mockExecutor{
		result: executor.ExecutorResult{
			Success:      true,
			FilesChanged: []string{"foo.go", "bar.go"},
		},
	}
	s := NewExecuteStage(exec, time.Minute)
	state := &PipelineState{
		JobID: "j1",
		Issue: GitHubIssue{Number: 1},
		Repo:  RepoContext{LocalPath: "/tmp/repo"},
		Plan:  "do the thing",
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Len(t, state.Changes, 2)
	assert.Equal(t, "foo.go", state.Changes[0].Path)
	assert.Equal(t, "modified", state.Changes[0].Action)
}

func TestExecuteStage_TimedOut(t *testing.T) {
	exec := &mockExecutor{
		result: executor.ExecutorResult{TimedOut: true},
	}
	s := NewExecuteStage(exec, time.Minute)
	state := &PipelineState{Issue: GitHubIssue{Number: 1}}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Output, "timed out")
}

func TestExecuteStage_ExecutionFailed(t *testing.T) {
	exec := &mockExecutor{
		result: executor.ExecutorResult{
			Success: false,
			Stderr:  "compilation error",
		},
	}
	s := NewExecuteStage(exec, time.Minute)
	state := &PipelineState{Issue: GitHubIssue{Number: 1}}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Output, "compilation error")
}

func TestExecuteStage_ExecutorError(t *testing.T) {
	exec := &mockExecutor{err: assert.AnError}
	s := NewExecuteStage(exec, time.Minute)
	state := &PipelineState{Issue: GitHubIssue{Number: 1}}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "executor run")
}
