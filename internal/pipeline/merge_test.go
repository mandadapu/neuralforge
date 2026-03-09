package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeStageName(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, false)
	assert.Equal(t, "merge", s.Name())
}

func TestMergeStageAutoMergeDisabled(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, false)
	state := &PipelineState{PRNumber: 5}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "Auto-merge disabled")
}

func TestMergeStageNoPR(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, true)
	state := &PipelineState{PRNumber: 0}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "No PR to merge")
}

func TestMergeStageRequestedChanges(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, true)
	state := &PipelineState{
		PRNumber:    5,
		ReviewNotes: "REQUEST_CHANGES: please fix the error handling",
		Repo:        RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Output, "merge blocked")
}

func TestMergeStageSuccess(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, true)
	state := &PipelineState{
		PRNumber:    5,
		ReviewNotes: "APPROVE: looks good",
		Repo:        RepoContext{FullName: "owner/repo"},
		Issue:       GitHubIssue{Number: 3},
	}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.True(t, state.Merged)
	assert.Contains(t, result.Output, "PR #5 merged")
}

func TestMergeStageMergePRError(t *testing.T) {
	mock := &mockGitHubClient{mergePRErr: errors.New("merge conflict detected")}
	s := NewMergeStage(mock, true)
	state := &PipelineState{
		PRNumber:    5,
		ReviewNotes: "APPROVE",
		Repo:        RepoContext{FullName: "owner/repo"},
		Issue:       GitHubIssue{Number: 3},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "merge conflict detected")
}

func TestMergeStageInvalidRepoName(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, true)
	state := &PipelineState{
		PRNumber:    5,
		ReviewNotes: "APPROVE",
		Repo:        RepoContext{FullName: "noslash"},
		Issue:       GitHubIssue{Number: 1},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid repo name")
}
