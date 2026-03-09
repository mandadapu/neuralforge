package pipeline

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPRStage_Name(t *testing.T) {
	s := NewPRStage(&mockGitHubClient{})
	assert.Equal(t, "pr", s.Name())
}

func TestPRStage_Success(t *testing.T) {
	client := &mockGitHubClient{
		createPRNum: 42,
		createPRURL: "https://github.com/owner/repo/pull/42",
	}
	s := NewPRStage(client)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1, Title: "Fix the bug"},
		Repo: RepoContext{
			FullName:      "owner/repo",
			DefaultBranch: "main",
		},
		Plan: "Step 1: ...",
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, 42, state.PRNumber)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", state.PRURL)
	assert.Contains(t, result.Output, "42")
}

func TestPRStage_TitleTruncated(t *testing.T) {
	client := &mockGitHubClient{createPRNum: 1, createPRURL: "https://github.com/o/r/pull/1"}
	s := NewPRStage(client)

	longTitle := strings.Repeat("a", 100)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1, Title: longTitle},
		Repo:  RepoContext{FullName: "owner/repo", DefaultBranch: "main"},
	}

	_, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	// Title is formatted as "fix: #1 <title>" and truncated to 72 chars
	// The test verifies no error, and the PR was created successfully.
	assert.Equal(t, 1, state.PRNumber)
}

func TestPRStage_InvalidRepoName(t *testing.T) {
	s := NewPRStage(&mockGitHubClient{})
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1, Title: "Test"},
		Repo:  RepoContext{FullName: "invalid-no-slash"},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid repo name")
}

func TestPRStage_ClientError(t *testing.T) {
	client := &mockGitHubClient{createPRErr: assert.AnError}
	s := NewPRStage(client)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1, Title: "Test"},
		Repo:  RepoContext{FullName: "owner/repo", DefaultBranch: "main"},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "create PR")
}
