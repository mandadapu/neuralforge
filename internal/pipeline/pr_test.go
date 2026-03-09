package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPRStageName(t *testing.T) {
	s := NewPRStage(&mockGitHubClient{})
	assert.Equal(t, "pr", s.Name())
}

func TestPRStageSuccess(t *testing.T) {
	mock := &mockGitHubClient{
		createPRNumber: 99,
		createPRURL:    "https://github.com/owner/repo/pull/99",
	}
	s := NewPRStage(mock)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 7, Title: "Fix the login bug"},
		Repo:  RepoContext{FullName: "owner/repo", DefaultBranch: "main"},
		Plan:  "detailed plan here",
		Cost:  0.05,
	}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, 99, state.PRNumber)
	assert.Equal(t, "https://github.com/owner/repo/pull/99", state.PRURL)
	assert.Contains(t, result.Output, "PR #99")
}

func TestPRStageGitHubError(t *testing.T) {
	mock := &mockGitHubClient{createPRErr: errors.New("rate limited by GitHub")}
	s := NewPRStage(mock)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1, Title: "Test"},
		Repo:  RepoContext{FullName: "owner/repo", DefaultBranch: "main"},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "rate limited by GitHub")
}

func TestPRStageInvalidRepoName(t *testing.T) {
	s := NewPRStage(&mockGitHubClient{})
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1, Title: "Test"},
		Repo:  RepoContext{FullName: "noslash"},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid repo name")
}

func TestPRStageTitleTruncated(t *testing.T) {
	mock := &mockGitHubClient{createPRNumber: 1, createPRURL: "https://github.com/owner/repo/pull/1"}
	s := NewPRStage(mock)
	// The title will be prefixed with "fix: #1 " (8 chars) + this long title
	longTitle := "This is an extremely long issue title that exceeds the PR title character limit set at seventy-two"
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1, Title: longTitle},
		Repo:  RepoContext{FullName: "owner/repo", DefaultBranch: "main"},
	}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
}

func TestPRStageSetsStateCorrectly(t *testing.T) {
	mock := &mockGitHubClient{createPRNumber: 5, createPRURL: "https://github.com/owner/repo/pull/5"}
	s := NewPRStage(mock)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 123, Title: "Test"},
		Repo:  RepoContext{FullName: "owner/repo", DefaultBranch: "main"},
	}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, 5, state.PRNumber)
	assert.Equal(t, "https://github.com/owner/repo/pull/5", state.PRURL)
}
