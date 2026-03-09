package llm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLLM struct {
	name     string
	resp     CompletionResponse
	err      error
	streamCh <-chan StreamChunk
	streamErr error
}

func (m *mockLLM) Name() string { return m.name }

func (m *mockLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	return m.resp, m.err
}

func (m *mockLLM) StreamComplete(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
	return m.streamCh, m.streamErr
}

func TestAuditedLLM_Name(t *testing.T) {
	mock := &mockLLM{name: "test-provider"}
	audited := NewAudited(mock)
	assert.Equal(t, "test-provider", audited.Name())
}

func TestAuditedLLM_Complete_success(t *testing.T) {
	want := CompletionResponse{
		Content:      "hello",
		Model:        "gpt-4",
		InputTokens:  10,
		OutputTokens: 5,
		Cost:         0.001,
	}
	mock := &mockLLM{name: "openai", resp: want}
	audited := NewAudited(mock)

	got, err := audited.Complete(context.Background(), CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestAuditedLLM_Complete_error(t *testing.T) {
	sentinel := errors.New("upstream error")
	mock := &mockLLM{name: "openai", err: sentinel}
	audited := NewAudited(mock)

	_, err := audited.Complete(context.Background(), CompletionRequest{})
	assert.ErrorIs(t, err, sentinel)
}

func TestAuditedLLM_StreamComplete(t *testing.T) {
	ch := make(chan StreamChunk)
	mock := &mockLLM{name: "claude", streamCh: ch}
	audited := NewAudited(mock)

	got, err := audited.StreamComplete(context.Background(), CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, ch, got)
}
