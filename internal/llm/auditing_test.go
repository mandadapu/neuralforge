package llm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubLLM is a minimal LLM mock for offline tests.
type stubLLM struct {
	name         string
	completeResp CompletionResponse
	completeErr  error
	streamCh     chan StreamChunk
	streamErr    error
}

func (s *stubLLM) Name() string { return s.name }

func (s *stubLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	return s.completeResp, s.completeErr
}

func (s *stubLLM) StreamComplete(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	if s.streamCh != nil {
		return s.streamCh, nil
	}
	ch := make(chan StreamChunk)
	close(ch)
	return ch, nil
}

func TestAuditingLLM_Name(t *testing.T) {
	stub := &stubLLM{name: "test-provider"}
	a := NewAuditingLLM(stub)
	assert.Equal(t, "test-provider", a.Name())
}

func TestAuditingLLM_Complete_Success(t *testing.T) {
	want := CompletionResponse{
		Content:      "hello",
		Model:        "gpt-4",
		InputTokens:  10,
		OutputTokens: 5,
		Cost:         0.001,
	}
	stub := &stubLLM{name: "openai", completeResp: want}
	a := NewAuditingLLM(stub)

	got, err := a.Complete(context.Background(), CompletionRequest{Model: "gpt-4"})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestAuditingLLM_Complete_Error(t *testing.T) {
	sentinelErr := errors.New("api unavailable")
	stub := &stubLLM{name: "openai", completeErr: sentinelErr}
	a := NewAuditingLLM(stub)

	_, err := a.Complete(context.Background(), CompletionRequest{Model: "gpt-4"})
	require.ErrorIs(t, err, sentinelErr)
}

func TestAuditingLLM_Complete_CorrelationID(t *testing.T) {
	want := CompletionResponse{Model: "gpt-4", InputTokens: 1, OutputTokens: 1}
	stub := &stubLLM{name: "openai", completeResp: want}
	a := NewAuditingLLM(stub)

	ctx := WithCorrelationID(context.Background(), "txn-abc-123")
	got, err := a.Complete(ctx, CompletionRequest{Model: "gpt-4"})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestAuditingLLM_StreamComplete(t *testing.T) {
	stub := &stubLLM{name: "claude"}
	a := NewAuditingLLM(stub)

	ch, err := a.StreamComplete(context.Background(), CompletionRequest{Model: "claude-3"})
	require.NoError(t, err)
	require.NotNil(t, ch)
	// Drain the wrapped channel so the audit goroutine completes.
	for range ch {
	}
}

func TestAuditingLLM_StreamComplete_DeferredAudit(t *testing.T) {
	// Verify that data flows through the wrapping channel correctly.
	inner := make(chan StreamChunk, 3)
	inner <- StreamChunk{Content: "a"}
	inner <- StreamChunk{Content: "b"}
	inner <- StreamChunk{Done: true}
	close(inner)

	stub := &stubLLM{name: "claude", streamCh: inner}
	a := NewAuditingLLM(stub)

	ctx := WithCorrelationID(context.Background(), "stream-corr-1")
	ch, err := a.StreamComplete(ctx, CompletionRequest{Model: "claude-3"})
	require.NoError(t, err)

	var got []StreamChunk
	for chunk := range ch {
		got = append(got, chunk)
	}
	require.Len(t, got, 3)
	assert.Equal(t, "a", got[0].Content)
	assert.Equal(t, "b", got[1].Content)
	assert.True(t, got[2].Done)
}

func TestAuditingLLM_StreamComplete_Error(t *testing.T) {
	sentinelErr := errors.New("stream error")
	stub := &stubLLM{name: "claude", streamErr: sentinelErr}
	a := NewAuditingLLM(stub)

	_, err := a.StreamComplete(context.Background(), CompletionRequest{Model: "claude-3"})
	require.ErrorIs(t, err, sentinelErr)
}

func TestAuditingLLM_StreamComplete_ChunkError(t *testing.T) {
	// Verify stream-level error (error in a chunk) is propagated and logged.
	chunkErr := errors.New("mid-stream failure")
	inner := make(chan StreamChunk, 2)
	inner <- StreamChunk{Content: "partial"}
	inner <- StreamChunk{Error: chunkErr}
	close(inner)

	stub := &stubLLM{name: "claude", streamCh: inner}
	a := NewAuditingLLM(stub)

	ch, err := a.StreamComplete(context.Background(), CompletionRequest{Model: "claude-3"})
	require.NoError(t, err) // channel open; error is inside a chunk

	var got []StreamChunk
	for chunk := range ch {
		got = append(got, chunk)
	}
	require.Len(t, got, 2)
	assert.ErrorIs(t, got[1].Error, chunkErr)
}

func TestCorrelationIDFromContext_Missing(t *testing.T) {
	id := correlationIDFromContext(context.Background())
	assert.Equal(t, "", id)
}

func TestCorrelationIDFromContext_Present(t *testing.T) {
	ctx := WithCorrelationID(context.Background(), "req-42")
	id := correlationIDFromContext(ctx)
	assert.Equal(t, "req-42", id)
}

func TestAuditingLLM_ImplementsLLMInterface(t *testing.T) {
	stub := &stubLLM{name: "test"}
	var _ LLM = NewAuditingLLM(stub)
}
