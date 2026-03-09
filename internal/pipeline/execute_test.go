package pipeline

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/mandadapu/neuralforge/internal/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecuteStageName(t *testing.T) {
	s := NewExecuteStage(&mockExecutor{}, time.Minute)
	assert.Equal(t, "execute", s.Name())
}

func TestExecuteStageSuccess(t *testing.T) {
	mock := &mockExecutor{
		result: executor.ExecutorResult{
			Success:      true,
			FilesChanged: []string{"foo.go", "bar.go"},
		},
	}
	s := NewExecuteStage(mock, time.Minute)
	state := &PipelineState{
		JobID: "job-1",
		Issue: GitHubIssue{Number: 42},
		Repo:  RepoContext{LocalPath: "/tmp/repo"},
		Plan:  "do the thing",
	}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Len(t, state.Changes, 2)
	assert.Equal(t, "foo.go", state.Changes[0].Path)
	assert.Equal(t, "modified", state.Changes[0].Action)
	assert.Equal(t, "bar.go", state.Changes[1].Path)
}

func TestExecuteStageTimedOut(t *testing.T) {
	mock := &mockExecutor{
		result: executor.ExecutorResult{TimedOut: true},
	}
	s := NewExecuteStage(mock, time.Minute)
	state := &PipelineState{Issue: GitHubIssue{Number: 1}}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Output, "timed out")
}

func TestExecuteStageFailed(t *testing.T) {
	mock := &mockExecutor{
		result: executor.ExecutorResult{
			Success: false,
			Stderr:  "compilation error: undefined foo",
		},
	}
	s := NewExecuteStage(mock, time.Minute)
	state := &PipelineState{Issue: GitHubIssue{Number: 1}}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Output, "compilation error")
}

func TestExecuteStageExecutorError(t *testing.T) {
	mock := &mockExecutor{err: errors.New("docker daemon not running")}
	s := NewExecuteStage(mock, time.Minute)
	state := &PipelineState{Issue: GitHubIssue{Number: 1}}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "docker daemon not running")
}

func TestExecuteStageNoFilesChanged(t *testing.T) {
	mock := &mockExecutor{
		result: executor.ExecutorResult{
			Success:      true,
			FilesChanged: []string{},
		},
	}
	s := NewExecuteStage(mock, time.Minute)
	state := &PipelineState{Issue: GitHubIssue{Number: 1}}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Empty(t, state.Changes)
}
