package llm

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureHandler is a simple slog.Handler that records log records for assertions.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(name string) slog.Handler       { return h }

func (h *captureHandler) attrMap(idx int) map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	m := make(map[string]any)
	h.records[idx].Attrs(func(a slog.Attr) bool {
		m[a.Key] = a.Value.Any()
		return true
	})
	return m
}

func (h *captureHandler) len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.records)
}

func (h *captureHandler) level(idx int) slog.Level {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.records[idx].Level
}

func (h *captureHandler) message(idx int) string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.records[idx].Message
}

// mockLLM is a test double for the LLM interface.
type mockLLM struct {
	name     string
	completeFunc func(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
	streamFunc   func(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
}

func (m *mockLLM) Name() string { return m.name }

func (m *mockLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, req)
	}
	return CompletionResponse{
		Content:      "test response",
		Model:        "test-model",
		InputTokens:  10,
		OutputTokens: 20,
		Cost:         0.001,
	}, nil
}

func (m *mockLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req)
	}
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Content: "hello"}
	ch <- StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

func TestAuditedLLM_Complete_LogsFields(t *testing.T) {
	h := &captureHandler{}
	old := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(old)

	mock := &mockLLM{name: "mock-provider"}
	audited := NewAudited(mock)

	req := CompletionRequest{
		System:   "you are helpful",
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	}
	ctx := WithActor(context.Background(), "user-42")

	resp, err := audited.Complete(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, "test response", resp.Content)

	require.Equal(t, 1, h.len())
	assert.Equal(t, slog.LevelInfo, h.level(0))
	assert.Equal(t, "llm audit: Complete", h.message(0))

	attrs := h.attrMap(0)
	assert.Equal(t, "mock-provider", attrs["provider"])
	assert.Equal(t, "test-model", attrs["model"])
	assert.NotNil(t, attrs["input_tokens"])
	assert.NotNil(t, attrs["output_tokens"])
	assert.NotNil(t, attrs["cost_usd"])
	assert.NotNil(t, attrs["latency_ms"])
	assert.Equal(t, "user-42", attrs["actor"])
	assert.NotEmpty(t, attrs["prompt_hash"])
}

func TestAuditedLLM_Complete_LogsError(t *testing.T) {
	h := &captureHandler{}
	old := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(old)

	wantErr := errors.New("backend failure")
	mock := &mockLLM{
		name: "mock-provider",
		completeFunc: func(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
			return CompletionResponse{}, wantErr
		},
	}
	audited := NewAudited(mock)

	_, err := audited.Complete(context.Background(), CompletionRequest{})
	assert.ErrorIs(t, err, wantErr)

	require.Equal(t, 1, h.len())
	assert.Equal(t, slog.LevelError, h.level(0))
	assert.Equal(t, "llm audit: Complete failed", h.message(0))
}

func TestAuditedLLM_Complete_ActorUnknownWhenNotSet(t *testing.T) {
	h := &captureHandler{}
	old := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(old)

	audited := NewAudited(&mockLLM{name: "p"})
	_, err := audited.Complete(context.Background(), CompletionRequest{})
	require.NoError(t, err)

	attrs := h.attrMap(0)
	assert.Equal(t, "unknown", attrs["actor"])
}

func TestAuditedLLM_Complete_PromptHashIsStable(t *testing.T) {
	req := CompletionRequest{
		System:   "sys",
		Messages: []Message{{Role: RoleUser, Content: "msg"}},
	}
	h1 := promptHash(req)
	h2 := promptHash(req)
	assert.Equal(t, h1, h2)
	assert.Len(t, h1, 16)
}

func TestAuditedLLM_StreamComplete_LogsStartAndCompletion(t *testing.T) {
	h := &captureHandler{}
	old := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(old)

	mock := &mockLLM{name: "mock-provider"}
	audited := NewAudited(mock)

	ctx := WithActor(context.Background(), "stream-user")
	ch, err := audited.StreamComplete(ctx, CompletionRequest{System: "s", Model: "test-stream-model"})
	require.NoError(t, err)

	// drain channel
	for range ch {
	}

	// expect start + completion log
	require.Equal(t, 2, h.len())
	assert.Equal(t, "llm audit: StreamComplete started", h.message(0))
	assert.Equal(t, slog.LevelInfo, h.level(0))

	startAttrs := h.attrMap(0)
	assert.Equal(t, "mock-provider", startAttrs["provider"])
	assert.Equal(t, "test-stream-model", startAttrs["model"])
	assert.Equal(t, "stream-user", startAttrs["actor"])
	assert.NotEmpty(t, startAttrs["prompt_hash"])

	assert.Equal(t, "llm audit: StreamComplete completed", h.message(1))
	assert.Equal(t, slog.LevelInfo, h.level(1))

	doneAttrs := h.attrMap(1)
	assert.Equal(t, "mock-provider", doneAttrs["provider"])
	assert.Equal(t, "test-stream-model", doneAttrs["model"])
	assert.NotNil(t, doneAttrs["chunks"])
	assert.NotNil(t, doneAttrs["latency_ms"])
}

func TestAuditedLLM_StreamComplete_LogsInitError(t *testing.T) {
	h := &captureHandler{}
	old := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(old)

	wantErr := errors.New("stream init failure")
	mock := &mockLLM{
		name: "mock-provider",
		streamFunc: func(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
			return nil, wantErr
		},
	}
	audited := NewAudited(mock)

	_, err := audited.StreamComplete(context.Background(), CompletionRequest{})
	assert.ErrorIs(t, err, wantErr)

	require.Equal(t, 2, h.len())
	assert.Equal(t, "llm audit: StreamComplete started", h.message(0))
	assert.Equal(t, slog.LevelError, h.level(1))
	assert.Equal(t, "llm audit: StreamComplete failed", h.message(1))
}

func TestAuditedLLM_StreamComplete_LogsChunkError(t *testing.T) {
	h := &captureHandler{}
	old := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(old)

	chunkErr := errors.New("chunk error")
	mock := &mockLLM{
		name: "mock-provider",
		streamFunc: func(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
			ch := make(chan StreamChunk, 2)
			ch <- StreamChunk{Content: "partial"}
			ch <- StreamChunk{Error: chunkErr, Done: true}
			close(ch)
			return ch, nil
		},
	}
	audited := NewAudited(mock)

	ch, err := audited.StreamComplete(context.Background(), CompletionRequest{})
	require.NoError(t, err)
	for range ch {
	}

	require.Equal(t, 2, h.len())
	assert.Equal(t, slog.LevelError, h.level(1))
	assert.Equal(t, "llm audit: StreamComplete completed with error", h.message(1))
}

func TestAuditedLLM_Name_DelegatesInner(t *testing.T) {
	mock := &mockLLM{name: "my-backend"}
	audited := NewAudited(mock)
	assert.Equal(t, "my-backend", audited.Name())
}

func TestWithActor(t *testing.T) {
	ctx := WithActor(context.Background(), "alice")
	assert.Equal(t, "alice", actorFromCtx(ctx))
}

func TestActorFromCtx_NoActor(t *testing.T) {
	assert.Equal(t, "unknown", actorFromCtx(context.Background()))
}
