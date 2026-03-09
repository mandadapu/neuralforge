package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/mandadapu/neuralforge/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewStageName(t *testing.T) {
	s := NewReviewStage(&mockLLM{}, &mockGitHubClient{})
	assert.Equal(t, "review", s.Name())
}

func TestReviewStageNoPR(t *testing.T) {
	s := NewReviewStage(&mockLLM{}, &mockGitHubClient{})
	state := &PipelineState{PRNumber: 0}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "No PR to review")
}

func TestReviewStageApprove(t *testing.T) {
	mockL := &mockLLM{
		resp: llm.CompletionResponse{Content: "APPROVE: changes look good", Cost: 0.02},
	}
	s := NewReviewStage(mockL, &mockGitHubClient{})
	state := &PipelineState{
		PRNumber: 5,
		Issue:    GitHubIssue{Number: 3, Title: "Fix it"},
		Repo:     RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Contains(t, result.Output, "APPROVE")
	assert.Equal(t, "APPROVE: changes look good", state.ReviewNotes)
	assert.InDelta(t, 0.02, state.Cost, 1e-9)
}

func TestReviewStageRequestChanges(t *testing.T) {
	mockL := &mockLLM{
		resp: llm.CompletionResponse{Content: "REQUEST_CHANGES: error handling is missing"},
	}
	s := NewReviewStage(mockL, &mockGitHubClient{})
	state := &PipelineState{
		PRNumber: 5,
		Issue:    GitHubIssue{Number: 3, Title: "Test"},
		Repo:     RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Contains(t, result.Output, "REQUEST_CHANGES")
}

func TestReviewStageLLMError(t *testing.T) {
	mockL := &mockLLM{err: errors.New("context deadline exceeded")}
	s := NewReviewStage(mockL, &mockGitHubClient{})
	state := &PipelineState{
		PRNumber: 5,
		Issue:    GitHubIssue{Number: 3},
		Repo:     RepoContext{FullName: "owner/repo"},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}

func TestReviewStageReviewPostError(t *testing.T) {
	mockL := &mockLLM{resp: llm.CompletionResponse{Content: "APPROVE: looks great"}}
	mockGH := &mockGitHubClient{createReviewErr: errors.New("github api error")}
	s := NewReviewStage(mockL, mockGH)
	state := &PipelineState{
		PRNumber: 5,
		Issue:    GitHubIssue{Number: 3},
		Repo:     RepoContext{FullName: "owner/repo"},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "github api error")
}

func TestReviewStageInvalidRepoName(t *testing.T) {
	mockL := &mockLLM{resp: llm.CompletionResponse{Content: "APPROVE"}}
	s := NewReviewStage(mockL, &mockGitHubClient{})
	state := &PipelineState{
		PRNumber: 5,
		Issue:    GitHubIssue{Number: 1},
		Repo:     RepoContext{FullName: "noslash"},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid repo name")
}

func TestReviewStageWithChangesAndTestResults(t *testing.T) {
	mockL := &mockLLM{
		resp: llm.CompletionResponse{Content: "APPROVE: all tests pass"},
	}
	s := NewReviewStage(mockL, &mockGitHubClient{})
	state := &PipelineState{
		PRNumber: 5,
		Issue:    GitHubIssue{Number: 3, Title: "Add feature"},
		Repo:     RepoContext{FullName: "owner/repo"},
		Changes: []FileChange{
			{Path: "main.go", Action: "modified", Diff: "+func NewFeature() {}"},
		},
		TestResults: &TestReport{Passed: true, Output: "ok github.com/owner/repo 0.5s"},
	}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
}
