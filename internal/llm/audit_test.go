package llm

import (
	"context"
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
	return nil, nil
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
	llmClient := New("openai", "key", "gpt-4")
	audited, ok := llmClient.(*AuditingLLM)
	assert.True(t, ok)
	assert.Equal(t, "openai", audited.Name())
}

func TestNew_DefaultClaude(t *testing.T) {
	llmClient := New("claude", "key", "claude-3-opus-20240229")
	audited, ok := llmClient.(*AuditingLLM)
	assert.True(t, ok)
	assert.Equal(t, "claude", audited.Name())
}
