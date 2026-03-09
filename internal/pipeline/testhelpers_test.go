package pipeline

import (
	"context"
	"fmt"

	"github.com/mandadapu/neuralforge/internal/executor"
	"github.com/mandadapu/neuralforge/internal/llm"
)

// mockLLM implements llm.LLM for testing pipeline stages.
type mockLLM struct {
	response llm.CompletionResponse
	err      error
	lastReq  llm.CompletionRequest
}

func (m *mockLLM) Complete(ctx context.Context, req llm.CompletionRequest) (llm.CompletionResponse, error) {
	m.lastReq = req
	return m.response, m.err
}

func (m *mockLLM) StreamComplete(ctx context.Context, req llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockLLM) Name() string { return "mock" }

// mockGitHubClient implements pipeline.GitHubClient for testing.
type mockGitHubClient struct {
	createPRFn    func(ctx context.Context, owner, repo, title, body, head, base string) (int, string, error)
	createReviewFn func(ctx context.Context, owner, repo string, prNumber int, body, event string) error
	mergePRFn     func(ctx context.Context, owner, repo string, number int, message string) error
	commentFn     func(ctx context.Context, owner, repo string, number int, body string) error
}

func (m *mockGitHubClient) CreatePR(ctx context.Context, owner, repo, title, body, head, base string) (int, string, error) {
	if m.createPRFn != nil {
		return m.createPRFn(ctx, owner, repo, title, body, head, base)
	}
	return 0, "", nil
}

func (m *mockGitHubClient) CreateReview(ctx context.Context, owner, repo string, prNumber int, body, event string) error {
	if m.createReviewFn != nil {
		return m.createReviewFn(ctx, owner, repo, prNumber, body, event)
	}
	return nil
}

func (m *mockGitHubClient) MergePR(ctx context.Context, owner, repo string, number int, message string) error {
	if m.mergePRFn != nil {
		return m.mergePRFn(ctx, owner, repo, number, message)
	}
	return nil
}

func (m *mockGitHubClient) CommentOnIssue(ctx context.Context, owner, repo string, number int, body string) error {
	if m.commentFn != nil {
		return m.commentFn(ctx, owner, repo, number, body)
	}
	return nil
}

// mockExecutor implements executor.Executor for testing.
type mockExecutor struct {
	result executor.ExecutorResult
	err    error
	lastJob executor.ExecutorJob
}

func (m *mockExecutor) Run(ctx context.Context, job executor.ExecutorJob) (executor.ExecutorResult, error) {
	m.lastJob = job
	return m.result, m.err
}

func (m *mockExecutor) Cleanup(ctx context.Context, jobID string) error {
	return nil
}

func (m *mockExecutor) Name() string { return "mock" }
