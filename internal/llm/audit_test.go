package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
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

func TestAuditingLLM_Complete_LogsAuditEntry(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	slog.SetDefault(logger)

	stub := &stubLLM{
		name: "test-provider",
		resp: CompletionResponse{
			Content:      "hello",
			Model:        "test-model",
			InputTokens:  10,
			OutputTokens: 5,
			Cost:         0.001,
		},
	}

	auditing := WithAudit(stub)
	assert.Equal(t, "test-provider", auditing.Name())

	resp, err := auditing.Complete(context.Background(), CompletionRequest{})
	require.NoError(t, err)
	assert.Equal(t, "hello", resp.Content)

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	assert.Equal(t, "llm_audit", entry["msg"])
	assert.Equal(t, "test-provider", entry["provider"])
	assert.Equal(t, "test-model", entry["model"])
	assert.EqualValues(t, 10, entry["input_tokens"])
	assert.EqualValues(t, 5, entry["output_tokens"])
	assert.Contains(t, entry, "latency_ms")
}
