package llm

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubLLM struct {
	name     string
	response CompletionResponse
	err      error
}

func (s *stubLLM) Name() string { return s.name }

func (s *stubLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	return s.response, s.err
}

func (s *stubLLM) StreamComplete(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)
	close(ch)
	return ch, s.err
}

func TestAuditedLLM_Complete(t *testing.T) {
	stub := &stubLLM{
		name: "test-provider",
		response: CompletionResponse{
			Content:      "hello",
			Model:        "test-model",
			InputTokens:  10,
			OutputTokens: 5,
			Cost:         0.001,
		},
	}

	audited := NewAudited(stub, slog.Default())
	assert.Equal(t, "test-provider", audited.Name())

	resp, err := audited.Complete(context.Background(), CompletionRequest{Model: "test-model"})
	require.NoError(t, err)
	assert.Equal(t, stub.response, resp)
}

func TestAuditedLLM_StreamComplete(t *testing.T) {
	stub := &stubLLM{name: "test-provider"}
	audited := NewAudited(stub, slog.Default())

	ch, err := audited.StreamComplete(context.Background(), CompletionRequest{Model: "test-model"})
	require.NoError(t, err)
	assert.NotNil(t, ch)
}
