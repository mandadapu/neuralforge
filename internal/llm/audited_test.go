package llm

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLLM struct {
	name         string
	completeResp CompletionResponse
	completeErr  error
	streamCh     chan StreamChunk
	streamErr    error
}

func (m *mockLLM) Name() string { return m.name }

func (m *mockLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	return m.completeResp, m.completeErr
}

func (m *mockLLM) StreamComplete(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
	return m.streamCh, m.streamErr
}

func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestAuditedLLM_Name(t *testing.T) {
	inner := &mockLLM{name: "test-provider"}
	a := NewAudited(inner)
	assert.Equal(t, "test-provider", a.Name())
}

func TestAuditedLLM_Complete_Success(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(newTestLogger(&buf))

	inner := &mockLLM{
		name: "openai",
		completeResp: CompletionResponse{
			Content:      "hello",
			Model:        "gpt-4",
			InputTokens:  10,
			OutputTokens: 5,
			Cost:         0.0012,
		},
	}
	a := NewAudited(inner)

	req := CompletionRequest{Model: "gpt-4", MaxTokens: 100}
	resp, err := a.Complete(context.Background(), req)

	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Content)

	log := buf.String()
	assert.Contains(t, log, "llm.complete")
	assert.Contains(t, log, "openai")
	assert.Contains(t, log, "gpt-4")
	assert.Contains(t, log, "input_tokens")
	assert.Contains(t, log, "output_tokens")
	assert.Contains(t, log, "cost_usd")
	assert.Contains(t, log, "latency_ms")
}

func TestAuditedLLM_Complete_Error(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(newTestLogger(&buf))

	inner := &mockLLM{
		name:        "claude",
		completeErr: errors.New("rate limit exceeded"),
	}
	a := NewAudited(inner)

	req := CompletionRequest{Model: "claude-3", MaxTokens: 100}
	_, err := a.Complete(context.Background(), req)

	require.Error(t, err)
	assert.EqualError(t, err, "rate limit exceeded")

	log := buf.String()
	assert.Contains(t, log, "llm.complete")
	assert.Contains(t, log, "claude")
	assert.Contains(t, log, "error")
	assert.True(t, strings.Contains(log, "rate limit exceeded"))
}

func TestAuditedLLM_StreamComplete(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(newTestLogger(&buf))

	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Content: "chunk", Done: true}
	close(ch)

	inner := &mockLLM{
		name:     "openai",
		streamCh: ch,
	}
	a := NewAudited(inner)

	req := CompletionRequest{Model: "gpt-4", MaxTokens: 100}
	resultCh, err := a.StreamComplete(context.Background(), req)

	require.NoError(t, err)
	assert.NotNil(t, resultCh)

	log := buf.String()
	assert.Contains(t, log, "llm.stream_complete")
	assert.Contains(t, log, "openai")
}

func TestNewAudited_NilPanics(t *testing.T) {
	assert.Panics(t, func() {
		NewAudited(nil)
	})
}
