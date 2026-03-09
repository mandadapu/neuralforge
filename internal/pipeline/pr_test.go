package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPRStage_Name(t *testing.T) {
	s := NewPRStage(&mockGitHubClient{})
	assert.Equal(t, "pr", s.Name())
}

func TestPRStage_CreatesPRWithCorrectFields(t *testing.T) {
	var capturedTitle, capturedBody, capturedHead, capturedBase string
	var capturedOwner, capturedRepo string

	client := &mockGitHubClient{
		createPRFn: func(ctx context.Context, owner, repo, title, body, head, base string) (int, string, error) {
			capturedOwner = owner
			capturedRepo = repo
			capturedTitle = title
			capturedBody = body
			capturedHead = head
			capturedBase = base
			return 42, "https://github.com/owner/repo/pull/42", nil
		},
	}

	s := NewPRStage(client)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 7, Title: "Fix the bug"},
		Repo: RepoContext{
			FullName:      "owner/repo",
			DefaultBranch: "main",
		},
		Plan: "Step 1: do this\nStep 2: do that",
		Cost: 0.0123,
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, "owner", capturedOwner)
	assert.Equal(t, "repo", capturedRepo)
	assert.Contains(t, capturedTitle, "#7")
	assert.Contains(t, capturedTitle, "Fix the bug")
	assert.Contains(t, capturedBody, "Automated fix for #7")
	assert.Equal(t, "neuralforge/issue-7", capturedHead)
	assert.Equal(t, "main", capturedBase)
	assert.Equal(t, 42, state.PRNumber)
	assert.Equal(t, "https://github.com/owner/repo/pull/42", state.PRURL)
}

func TestPRStage_TitleTruncatedAt72Chars(t *testing.T) {
	var capturedTitle string
	client := &mockGitHubClient{
		createPRFn: func(ctx context.Context, owner, repo, title, body, head, base string) (int, string, error) {
			capturedTitle = title
			return 1, "https://github.com/o/r/pull/1", nil
		},
	}

	s := NewPRStage(client)
	state := &PipelineState{
		Issue: GitHubIssue{
			Number: 1,
			Title:  "This is a very long title that should be truncated because it exceeds the limit",
		},
		Repo: RepoContext{FullName: "o/r", DefaultBranch: "main"},
	}

	_, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.LessOrEqual(t, len(capturedTitle), 72)
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

func TestPRStage_GitHubErrorPropagates(t *testing.T) {
	client := &mockGitHubClient{
		createPRFn: func(ctx context.Context, owner, repo, title, body, head, base string) (int, string, error) {
			return 0, "", errors.New("github API error")
		},
	}

	s := NewPRStage(client)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1, Title: "Test"},
		Repo:  RepoContext{FullName: "o/r", DefaultBranch: "main"},
	}

	_, err := s.Run(context.Background(), state)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "create PR")
}
