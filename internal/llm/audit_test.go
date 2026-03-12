package llm

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubLLM struct {
	name string
	resp CompletionResponse
	err  error
}

func (s *stubLLM) Name() string { return s.name }

func (s *stubLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	return s.resp, s.err
}

func (s *stubLLM) StreamComplete(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Content: "chunk", Done: true}
	close(ch)
	return ch, s.err
}

func TestAuditingLLM_Complete_PassThrough(t *testing.T) {
	want := CompletionResponse{
		Content:      "hello",
		Model:        "gpt-4",
		InputTokens:  10,
		OutputTokens: 5,
		Cost:         0.001,
	}
	stub := &stubLLM{name: "stub", resp: want}
	a := NewAuditingLLM(stub)

	got, err := a.Complete(context.Background(), CompletionRequest{Model: "gpt-4"})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestAuditingLLM_Complete_ErrorPassThrough(t *testing.T) {
	wantErr := errors.New("backend error")
	stub := &stubLLM{name: "stub", err: wantErr}
	a := NewAuditingLLM(stub)

	_, err := a.Complete(context.Background(), CompletionRequest{})
	assert.ErrorIs(t, err, wantErr)
}

func TestAuditingLLM_Name(t *testing.T) {
	stub := &stubLLM{name: "test-provider"}
	a := NewAuditingLLM(stub)
	assert.Equal(t, "test-provider", a.Name())
}

func TestAuditingLLM_StreamComplete(t *testing.T) {
	stub := &stubLLM{name: "stub"}
	a := NewAuditingLLM(stub)

	ch, err := a.StreamComplete(context.Background(), CompletionRequest{Model: "gpt-4"})
	require.NoError(t, err)

	chunk := <-ch
	assert.Equal(t, "chunk", chunk.Content)
	assert.True(t, chunk.Done)
}

func TestAuditingLLM_StreamComplete_InitError(t *testing.T) {
	wantErr := errors.New("stream init error")
	stub := &stubLLM{name: "stub", err: wantErr}
	a := NewAuditingLLM(stub)

	ch, err := a.StreamComplete(context.Background(), CompletionRequest{Model: "gpt-4"})
	assert.ErrorIs(t, err, wantErr)
	assert.Nil(t, ch)
}

func TestRedactError_Short(t *testing.T) {
	err := errors.New("short error")
	assert.Equal(t, "short error", redactError(err))
}

func TestRedactError_Truncated(t *testing.T) {
	long := strings.Repeat("x", maxErrorLen+50)
	result := redactError(errors.New(long))
	assert.LessOrEqual(t, len(result), maxErrorLen+len(" [truncated]"))
	assert.Contains(t, result, "[truncated]")
}

func TestRedactError_Nil(t *testing.T) {
	assert.Equal(t, "", redactError(nil))
}

func TestNew_DisallowedProvider(t *testing.T) {
	_, err := New("unknown-provider", "key", "model")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not permitted")
}

func TestNew_AllowedProviders(t *testing.T) {
	for _, provider := range []string{"openai", "claude", "anthropic"} {
		llm, err := New(provider, "key", "model")
		assert.NoError(t, err, "provider %q should be allowed", provider)
		assert.NotNil(t, llm)
	}
}
