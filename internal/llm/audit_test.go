package llm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubLLM struct {
	name     string
	resp     CompletionResponse
	err      error
	called   bool
}

func (s *stubLLM) Name() string { return s.name }

func (s *stubLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	s.called = true
	return s.resp, s.err
}

func (s *stubLLM) StreamComplete(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
	return nil, s.err
}

func TestAuditedLLM_Complete_delegates(t *testing.T) {
	want := CompletionResponse{
		Content:      "hello",
		Model:        "gpt-4",
		InputTokens:  10,
		OutputTokens: 5,
		Cost:         0.001,
	}
	stub := &stubLLM{name: "stub", resp: want}
	audited := NewAudited(stub)

	got, err := audited.Complete(context.Background(), CompletionRequest{Model: "gpt-4"})

	require.NoError(t, err)
	assert.Equal(t, want, got)
	assert.True(t, stub.called)
}

func TestAuditedLLM_Complete_propagates_error(t *testing.T) {
	stubErr := errors.New("backend error")
	stub := &stubLLM{name: "stub", err: stubErr}
	audited := NewAudited(stub)

	_, err := audited.Complete(context.Background(), CompletionRequest{})

	assert.ErrorIs(t, err, stubErr)
}

func TestAuditedLLM_Name(t *testing.T) {
	stub := &stubLLM{name: "myprovider"}
	assert.Equal(t, "myprovider", NewAudited(stub).Name())
}

func TestAuditedLLM_StreamComplete_propagates_error(t *testing.T) {
	stubErr := errors.New("stream error")
	stub := &stubLLM{name: "stub", err: stubErr}
	audited := NewAudited(stub)

	ch, err := audited.StreamComplete(context.Background(), CompletionRequest{})

	assert.Nil(t, ch)
	assert.ErrorIs(t, err, stubErr)
}
