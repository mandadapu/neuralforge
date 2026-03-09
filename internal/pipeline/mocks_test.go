package pipeline

import (
	"context"

	"github.com/mandadapu/neuralforge/internal/executor"
	"github.com/mandadapu/neuralforge/internal/llm"
)

// mockLLM implements llm.LLM for testing.
type mockLLM struct {
	resp llm.CompletionResponse
	err  error
}

func (m *mockLLM) Complete(_ context.Context, _ llm.CompletionRequest) (llm.CompletionResponse, error) {
	return m.resp, m.err
}

func (m *mockLLM) StreamComplete(_ context.Context, _ llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	return nil, nil
}

func (m *mockLLM) Name() string { return "mock" }

// mockExecutor implements executor.Executor for testing.
type mockExecutor struct {
	result executor.ExecutorResult
	err    error
}

func (m *mockExecutor) Run(_ context.Context, _ executor.ExecutorJob) (executor.ExecutorResult, error) {
	return m.result, m.err
}

func (m *mockExecutor) Cleanup(_ context.Context, _ string) error { return nil }

func (m *mockExecutor) Name() string { return "mock" }

// mockGitHubClient implements GitHubClient for testing.
type mockGitHubClient struct {
	createPRNumber  int
	createPRURL     string
	createPRErr     error
	createReviewErr error
	mergePRErr      error
	commentErr      error
}

func (m *mockGitHubClient) CreatePR(_ context.Context, _, _, _, _, _, _ string) (int, string, error) {
	return m.createPRNumber, m.createPRURL, m.createPRErr
}

func (m *mockGitHubClient) CreateReview(_ context.Context, _, _ string, _ int, _, _ string) error {
	return m.createReviewErr
}

func (m *mockGitHubClient) MergePR(_ context.Context, _, _ string, _ int, _ string) error {
	return m.mergePRErr
}

func (m *mockGitHubClient) CommentOnIssue(_ context.Context, _, _ string, _ int, _ string) error {
	return m.commentErr
}
