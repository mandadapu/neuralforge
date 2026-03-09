package llm

import (
	"context"
	"errors"
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
