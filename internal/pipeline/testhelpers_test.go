package pipeline

import (
	"context"

	"github.com/mandadapu/neuralforge/internal/executor"
	"github.com/mandadapu/neuralforge/internal/llm"
)

// mockLLM is a test double for llm.LLM.
type mockLLM struct {
	content string
	cost    float64
	err     error
}

func (m *mockLLM) Complete(_ context.Context, _ llm.CompletionRequest) (llm.CompletionResponse, error) {
	if m.err != nil {
		return llm.CompletionResponse{}, m.err
	}
	return llm.CompletionResponse{Content: m.content, Cost: m.cost}, nil
}

func (m *mockLLM) StreamComplete(_ context.Context, _ llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	panic("not implemented")
}

func (m *mockLLM) Name() string { return "mock" }

// mockGitHubClient is a test double for GitHubClient.
type mockGitHubClient struct {
	createPRNum int
	createPRURL string
	createPRErr error
	mergeErr    error
	reviewErr   error
	commentErr  error
}

func (m *mockGitHubClient) CreatePR(_ context.Context, _, _, _, _, _, _ string) (int, string, error) {
	return m.createPRNum, m.createPRURL, m.createPRErr
}

func (m *mockGitHubClient) MergePR(_ context.Context, _, _ string, _ int, _ string) error {
	return m.mergeErr
}

func (m *mockGitHubClient) CreateReview(_ context.Context, _, _ string, _ int, _, _ string) error {
	return m.reviewErr
}

func (m *mockGitHubClient) CommentOnIssue(_ context.Context, _, _ string, _ int, _ string) error {
	return m.commentErr
}

// mockExecutor is a test double for executor.Executor.
type mockExecutor struct {
	result executor.ExecutorResult
	err    error
}

func (m *mockExecutor) Run(_ context.Context, _ executor.ExecutorJob) (executor.ExecutorResult, error) {
	return m.result, m.err
}

func (m *mockExecutor) Cleanup(_ context.Context, _ string) error { return nil }

func (m *mockExecutor) Name() string { return "mock" }
