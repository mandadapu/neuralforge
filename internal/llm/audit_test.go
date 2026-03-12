package llm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubLLM struct {
	name       string
	resp       CompletionResponse
	err        error
	streamCh   chan StreamChunk
	streamErr  error
}

func (s *stubLLM) Name() string { return s.name }

func (s *stubLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	return s.resp, s.err
}

func (s *stubLLM) StreamComplete(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	if s.streamCh != nil {
		return s.streamCh, nil
	}
	// Default: return a closed channel (empty stream).
	ch := make(chan StreamChunk)
	close(ch)
	return ch, nil
}

func TestAuditingLLM_Complete(t *testing.T) {
	expected := CompletionResponse{
		Content:      "hello",
		Model:        "gpt-4",
		InputTokens:  10,
		OutputTokens: 5,
		Cost:         0.001,
	}
	stub := &stubLLM{name: "openai", resp: expected}
	audited := &AuditingLLM{inner: stub}

	resp, err := audited.Complete(context.Background(), CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, expected, resp)
}

func TestAuditingLLM_Name(t *testing.T) {
	stub := &stubLLM{name: "claude"}
	audited := &AuditingLLM{inner: stub}
	assert.Equal(t, "claude", audited.Name())
}

func TestNew_OpenAI(t *testing.T) {
	llmClient, err := New("openai", "key", "gpt-4")
	require.NoError(t, err)
	audited, ok := llmClient.(*AuditingLLM)
	assert.True(t, ok)
	assert.Equal(t, "openai", audited.Name())
}

func TestNew_Claude(t *testing.T) {
	llmClient, err := New("claude", "key", "claude-3-opus-20240229")
	require.NoError(t, err)
	audited, ok := llmClient.(*AuditingLLM)
	assert.True(t, ok)
	assert.Equal(t, "claude", audited.Name())
}

func TestNew_UnknownProvider(t *testing.T) {
	llmClient, err := New("gemini", "key", "model")
	assert.Nil(t, llmClient)
	assert.ErrorContains(t, err, "unknown provider")
}

func TestAuditingLLM_StreamComplete_PassesChunks(t *testing.T) {
	upstream := make(chan StreamChunk, 3)
	upstream <- StreamChunk{Content: "a"}
	upstream <- StreamChunk{Content: "b"}
	upstream <- StreamChunk{Content: "c", Done: true}
	close(upstream)

	stub := &stubLLM{name: "openai", streamCh: upstream}
	audited := &AuditingLLM{inner: stub}

	ch, err := audited.StreamComplete(context.Background(), CompletionRequest{})
	require.NoError(t, err)
	require.NotNil(t, ch)

	var received []StreamChunk
	for chunk := range ch {
		received = append(received, chunk)
	}
	require.Len(t, received, 3)
	assert.Equal(t, "a", received[0].Content)
	assert.Equal(t, "b", received[1].Content)
	assert.True(t, received[2].Done)
}

func TestAuditingLLM_StreamComplete_InitiationError(t *testing.T) {
	stub := &stubLLM{name: "openai", streamErr: errors.New("connection refused")}
	audited := &AuditingLLM{inner: stub}

	ch, err := audited.StreamComplete(context.Background(), CompletionRequest{})
	assert.Nil(t, ch)
	assert.ErrorContains(t, err, "connection refused")
}
