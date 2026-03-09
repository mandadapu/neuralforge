package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeStage_Name(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, false)
	assert.Equal(t, "merge", s.Name())
}

func TestMergeStage_AutoMergeDisabled(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, false)
	state := &PipelineState{PRNumber: 42}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "disabled")
}

func TestMergeStage_NoPR(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, true)
	state := &PipelineState{PRNumber: 0}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "No PR")
}

func TestMergeStage_ReviewRequestedChanges(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, true)
	state := &PipelineState{
		PRNumber:    42,
		ReviewNotes: "REQUEST_CHANGES: needs improvement",
		Repo:        RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Output, "merge blocked")
}

func TestMergeStage_InvalidRepoName(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, true)
	state := &PipelineState{
		PRNumber: 42,
		Repo:     RepoContext{FullName: "invalid"},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid repo name")
}

func TestMergeStage_Success(t *testing.T) {
	s := NewMergeStage(&mockGitHubClient{}, true)
	state := &PipelineState{
		PRNumber: 42,
		Issue:    GitHubIssue{Number: 1},
		Repo:     RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.True(t, state.Merged)
	assert.Contains(t, result.Output, "42")
}

func TestMergeStage_ClientError(t *testing.T) {
	client := &mockGitHubClient{mergeErr: assert.AnError}
	s := NewMergeStage(client, true)
	state := &PipelineState{
		PRNumber: 42,
		Issue:    GitHubIssue{Number: 1},
		Repo:     RepoContext{FullName: "owner/repo"},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "merge PR")
}
