package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewStage_Name(t *testing.T) {
	s := NewReviewStage(&mockLLM{}, &mockGitHubClient{})
	assert.Equal(t, "review", s.Name())
}

func TestReviewStage_NoPR(t *testing.T) {
	s := NewReviewStage(&mockLLM{content: "APPROVE looks good"}, &mockGitHubClient{})
	state := &PipelineState{PRNumber: 0}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "No PR")
}

func TestReviewStage_Approve(t *testing.T) {
	llmMock := &mockLLM{content: "APPROVE looks great", cost: 0.05}
	client := &mockGitHubClient{}
	s := NewReviewStage(llmMock, client)
	state := &PipelineState{
		PRNumber: 42,
		Issue:    GitHubIssue{Number: 1, Title: "Fix bug"},
		Repo:     RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, "APPROVE looks great", state.ReviewNotes)
	assert.InDelta(t, 0.05, state.Cost, 0.001)
}

func TestReviewStage_RequestChanges(t *testing.T) {
	llmMock := &mockLLM{content: "REQUEST_CHANGES missing tests"}
	s := NewReviewStage(llmMock, &mockGitHubClient{})
	state := &PipelineState{
		PRNumber: 42,
		Issue:    GitHubIssue{Number: 1},
		Repo:     RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Output, "REQUEST_CHANGES")
}

func TestReviewStage_LLMError(t *testing.T) {
	llmMock := &mockLLM{err: assert.AnError}
	s := NewReviewStage(llmMock, &mockGitHubClient{})
	state := &PipelineState{
		PRNumber: 42,
		Issue:    GitHubIssue{Number: 1},
		Repo:     RepoContext{FullName: "owner/repo"},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "review llm call")
}

func TestReviewStage_InvalidRepoName(t *testing.T) {
	llmMock := &mockLLM{content: "APPROVE looks good"}
	s := NewReviewStage(llmMock, &mockGitHubClient{})
	state := &PipelineState{
		PRNumber: 42,
		Issue:    GitHubIssue{Number: 1},
		Repo:     RepoContext{FullName: "bad"},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid repo name")
}

func TestReviewStage_CreateReviewError(t *testing.T) {
	llmMock := &mockLLM{content: "APPROVE looks good"}
	client := &mockGitHubClient{reviewErr: assert.AnError}
	s := NewReviewStage(llmMock, client)
	state := &PipelineState{
		PRNumber: 42,
		Issue:    GitHubIssue{Number: 1},
		Repo:     RepoContext{FullName: "owner/repo"},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "post review")
}
