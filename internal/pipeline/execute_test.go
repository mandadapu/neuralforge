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

func TestExecuteStage_Name(t *testing.T) {
	s := NewExecuteStage(&mockExecutor{}, time.Minute)
	assert.Equal(t, "execute", s.Name())
}

func TestExecuteStage_SuccessfulExecution(t *testing.T) {
	exec := &mockExecutor{
		result: executor.ExecutorResult{
			Success:      true,
			Stdout:       "done",
			FilesChanged: []string{"main.go", "foo_test.go"},
		},
	}

	s := NewExecuteStage(exec, 5*time.Minute)
	state := &PipelineState{
		JobID:  "job-1",
		Issue:  GitHubIssue{Number: 3},
		Repo:   RepoContext{LocalPath: "/tmp/repo"},
		Plan:   "implement the feature",
		Memory: "context here",
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Contains(t, result.Output, "2 files changed")
	assert.Len(t, state.Changes, 2)
	assert.Equal(t, "main.go", state.Changes[0].Path)
	assert.Equal(t, "modified", state.Changes[0].Action)
}

func TestExecuteStage_PassesJobFieldsToExecutor(t *testing.T) {
	exec := &mockExecutor{
		result: executor.ExecutorResult{Success: true},
	}

	s := NewExecuteStage(exec, 10*time.Minute)
	state := &PipelineState{
		JobID:  "my-job",
		Issue:  GitHubIssue{Number: 42},
		Repo:   RepoContext{LocalPath: "/repo/path"},
		Plan:   "my plan",
		Memory: "my memory",
	}

	_, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, "my-job", exec.lastJob.ID)
	assert.Equal(t, "/repo/path", exec.lastJob.RepoPath)
	assert.Equal(t, "neuralforge/issue-42", exec.lastJob.Branch)
	assert.Equal(t, "my plan", exec.lastJob.Prompt)
	assert.Equal(t, "my memory", exec.lastJob.Context)
	assert.Equal(t, 10*time.Minute, exec.lastJob.Timeout)
}

func TestExecuteStage_TimedOut(t *testing.T) {
	exec := &mockExecutor{
		result: executor.ExecutorResult{
			TimedOut: true,
		},
	}

	s := NewExecuteStage(exec, time.Minute)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Output, "timed out")
}

func TestExecuteStage_Failure(t *testing.T) {
	exec := &mockExecutor{
		result: executor.ExecutorResult{
			Success: false,
			Stderr:  "compilation failed",
		},
	}

	s := NewExecuteStage(exec, time.Minute)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Output, "compilation failed")
}

func TestExecuteStage_ExecutorErrorPropagates(t *testing.T) {
	exec := &mockExecutor{
		err: errors.New("executor crashed"),
	}

	s := NewExecuteStage(exec, time.Minute)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1},
	}

	_, err := s.Run(context.Background(), state)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "executor run")
}
