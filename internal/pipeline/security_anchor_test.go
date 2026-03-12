package pipeline

import (
	"context"
	"testing"

	"github.com/mandadapu/neuralforge/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLLM captures the last CompletionRequest for inspection.
type mockLLM struct {
	lastReq llm.CompletionRequest
	resp    llm.CompletionResponse
}

func (m *mockLLM) Complete(_ context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	m.lastReq = req
	return m.resp, nil
}

func (m *mockLLM) StreamComplete(_ context.Context, _ llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	ch := make(chan llm.StreamChunk, 1)
	ch <- llm.StreamChunk{Content: m.resp.Content, Done: true}
	close(ch)
	return ch, nil
}

func (m *mockLLM) Name() string { return "mock" }

// mockGitHubClient satisfies the GitHubClient interface for tests.
type mockGitHubClient struct{}

func (m *mockGitHubClient) CreatePR(_ context.Context, _, _, _, _, _, _ string) (int, string, error) {
	return 0, "", nil
}
func (m *mockGitHubClient) CreateReview(_ context.Context, _, _ string, _ int, _, _ string) error {
	return nil
}
func (m *mockGitHubClient) MergePR(_ context.Context, _, _ string, _ int, _ string) error {
	return nil
}
func (m *mockGitHubClient) CommentOnIssue(_ context.Context, _, _ string, _ int, _ string) error {
	return nil
}

func TestSecurityAnchorInSystemPrompts(t *testing.T) {
	tests := []struct {
		name  string
		setup func(mock *mockLLM) (Stage, *PipelineState)
	}{
		{
			name: "architect stage has SECURITY anchor",
			setup: func(mock *mockLLM) (Stage, *PipelineState) {
				mock.resp = llm.CompletionResponse{Content: "plan text"}
				stage := NewArchitectStage(mock)
				state := &PipelineState{
					Issue: GitHubIssue{Number: 1, Title: "test", Body: "body"},
				}
				return stage, state
			},
		},
		{
			name: "review stage has SECURITY anchor",
			setup: func(mock *mockLLM) (Stage, *PipelineState) {
				mock.resp = llm.CompletionResponse{Content: "APPROVE looks good"}
				stage := NewReviewStage(mock, &mockGitHubClient{})
				state := &PipelineState{
					PRNumber: 1,
					Issue:    GitHubIssue{Number: 1, Title: "test", Body: "body"},
					Plan:     "do the thing",
					Repo:     RepoContext{FullName: "owner/repo"},
				}
				return stage, state
			},
		},
		{
			name: "security stage has SECURITY anchor",
			setup: func(mock *mockLLM) (Stage, *PipelineState) {
				mock.resp = llm.CompletionResponse{Content: "no issues found"}
				stage := NewSecurityStage(mock)
				state := &PipelineState{
					Repo: RepoContext{FullName: "owner/repo"},
					Plan: "do the thing",
				}
				return stage, state
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockLLM{}
			stage, state := tt.setup(mock)
			_, err := stage.Run(context.Background(), state)
			require.NoError(t, err)
			assert.Contains(t, mock.lastReq.System, "SECURITY:")
		})
	}
}
