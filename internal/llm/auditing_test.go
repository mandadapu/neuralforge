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
	name        string
	completeResp CompletionResponse
	completeErr  error
	streamCh    chan StreamChunk
	streamErr   error
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

func TestAuditingLLM_StreamComplete(t *testing.T) {
	stub := &stubLLM{name: "claude"}
	a := NewAuditingLLM(stub)

	ch, err := a.StreamComplete(context.Background(), CompletionRequest{Model: "claude-3"})
	require.NoError(t, err)
	assert.NotNil(t, ch)
}

func TestAuditingLLM_StreamComplete_Error(t *testing.T) {
	sentinelErr := errors.New("stream error")
	stub := &stubLLM{name: "claude", streamErr: sentinelErr}
	a := NewAuditingLLM(stub)

	_, err := a.StreamComplete(context.Background(), CompletionRequest{Model: "claude-3"})
	require.ErrorIs(t, err, sentinelErr)
}

func TestAuditingLLM_ImplementsLLMInterface(t *testing.T) {
	stub := &stubLLM{name: "test"}
	var _ LLM = NewAuditingLLM(stub)
}
