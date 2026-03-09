package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeStage_Name(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, true)
	assert.Equal(t, "merge", s.Name())
}

func TestMergeStage_AutoMergeDisabled(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, false)
	state := &PipelineState{PRNumber: 5}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "Auto-merge disabled")
}

func TestMergeStage_NoPR(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, true)
	state := &PipelineState{PRNumber: 0}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "No PR")
}

func TestMergeStage_BlockedByRequestChanges(t *testing.T) {
	var mergeCalled bool
	client := &mockGitHubClient{
		mergePRFn: func(ctx context.Context, owner, repo string, number int, message string) error {
			mergeCalled = true
			return nil
		},
	}

	s := NewMergeStage(client, true)
	state := &PipelineState{
		PRNumber:    10,
		ReviewNotes: "REQUEST_CHANGES\nNeeds more work.",
		Repo:        RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Output, "merge blocked")
	assert.False(t, mergeCalled, "merge should not have been called")
}

func TestMergeStage_SuccessfulMerge(t *testing.T) {
	var capturedNumber int
	client := &mockGitHubClient{
		mergePRFn: func(ctx context.Context, owner, repo string, number int, message string) error {
			capturedNumber = number
			return nil
		},
	}

	s := NewMergeStage(client, true)
	state := &PipelineState{
		PRNumber: 7,
		Issue:    GitHubIssue{Number: 3},
		Repo:     RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.True(t, state.Merged)
	assert.Equal(t, 7, capturedNumber)
}

func TestMergeStage_InvalidRepoName(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, true)
	state := &PipelineState{
		PRNumber: 1,
		Repo:     RepoContext{FullName: "noslash"},
	}

	_, err := s.Run(context.Background(), state)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid repo name")
}

func TestMergeStage_GitHubErrorPropagates(t *testing.T) {
	client := &mockGitHubClient{
		mergePRFn: func(ctx context.Context, owner, repo string, number int, message string) error {
			return errors.New("merge conflict")
		},
	}

	s := NewMergeStage(client, true)
	state := &PipelineState{
		PRNumber: 1,
		Issue:    GitHubIssue{Number: 1},
		Repo:     RepoContext{FullName: "o/r"},
	}

	_, err := s.Run(context.Background(), state)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "merge PR")
}
