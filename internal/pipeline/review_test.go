package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/mandadapu/neuralforge/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewStage_Name(t *testing.T) {
	s := NewReviewStage(&mockLLM{}, &mockGitHubClient{})
	assert.Equal(t, "review", s.Name())
}

func TestReviewStage_SkipsWhenNoPR(t *testing.T) {
	s := NewReviewStage(&mockLLM{}, &mockGitHubClient{})
	state := &PipelineState{PRNumber: 0}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
}

func TestReviewStage_ApproveWhenLLMSaysApprove(t *testing.T) {
	ml := &mockLLM{
		response: llm.CompletionResponse{
			Content: "APPROVE\nLooks great, well done.",
			Cost:    0.01,
		},
	}

	var capturedEvent string
	client := &mockGitHubClient{
		createReviewFn: func(ctx context.Context, owner, repo string, prNumber int, body, event string) error {
			capturedEvent = event
			return nil
		},
	}

	s := NewReviewStage(ml, client)
	state := &PipelineState{
		PRNumber: 5,
		Issue:    GitHubIssue{Number: 3, Title: "Fix bug"},
		Repo:     RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, "APPROVE", capturedEvent)
	assert.Equal(t, "APPROVE\nLooks great, well done.", state.ReviewNotes)
}

func TestReviewStage_RequestChangesWhenLLMSaysRequestChanges(t *testing.T) {
	ml := &mockLLM{
		response: llm.CompletionResponse{
			Content: "REQUEST_CHANGES\nNeeds more error handling.",
		},
	}

	var capturedEvent string
	client := &mockGitHubClient{
		createReviewFn: func(ctx context.Context, owner, repo string, prNumber int, body, event string) error {
			capturedEvent = event
			return nil
		},
	}

	s := NewReviewStage(ml, client)
	state := &PipelineState{
		PRNumber: 5,
		Issue:    GitHubIssue{Number: 3, Title: "Fix bug"},
		Repo:     RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	assert.Equal(t, "REQUEST_CHANGES", capturedEvent)
}

func TestReviewStage_InvalidRepoName(t *testing.T) {
	ml := &mockLLM{
		response: llm.CompletionResponse{Content: "APPROVE ok"},
	}

	s := NewReviewStage(ml, &mockGitHubClient{})
	state := &PipelineState{
		PRNumber: 1,
		Issue:    GitHubIssue{Number: 1, Title: "Test"},
		Repo:     RepoContext{FullName: "noslash"},
	}

	_, err := s.Run(context.Background(), state)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid repo name")
}

func TestReviewStage_LLMErrorPropagates(t *testing.T) {
	ml := &mockLLM{
		err: errors.New("LLM error"),
	}

	s := NewReviewStage(ml, &mockGitHubClient{})
	state := &PipelineState{
		PRNumber: 1,
		Issue:    GitHubIssue{Number: 1, Title: "Test"},
		Repo:     RepoContext{FullName: "o/r"},
	}

	_, err := s.Run(context.Background(), state)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "review llm call")
}

func TestReviewStage_GitHubErrorPropagates(t *testing.T) {
	ml := &mockLLM{
		response: llm.CompletionResponse{Content: "APPROVE looks good"},
	}
	client := &mockGitHubClient{
		createReviewFn: func(ctx context.Context, owner, repo string, prNumber int, body, event string) error {
			return errors.New("github API error")
		},
	}

	s := NewReviewStage(ml, client)
	state := &PipelineState{
		PRNumber: 1,
		Issue:    GitHubIssue{Number: 1, Title: "Test"},
		Repo:     RepoContext{FullName: "o/r"},
	}

	_, err := s.Run(context.Background(), state)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "post review")
}
