package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubLLM struct {
	name    string
	resp    CompletionResponse
	err     error
	streamErr error
}

func (s *stubLLM) Name() string { return s.name }

func (s *stubLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	return s.resp, s.err
}

func (s *stubLLM) StreamComplete(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
	return nil, s.streamErr
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
	// error field must be the sanitized status string, not the raw error value
	assert.Equal(t, "none", entry["error"])
}

func TestAuditingLLM_Complete_SanitizesError(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	sensitiveMsg := "context: api_key=sk-secret123 user_id=42"
	stub := &stubLLM{
		name: "test-provider",
		err:  errors.New(sensitiveMsg),
	}

	auditing := WithAudit(stub)
	_, _ = auditing.Complete(context.Background(), CompletionRequest{})

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	// Raw error message must not appear in logs.
	assert.Equal(t, "error", entry["error"])
	assert.NotContains(t, buf.String(), sensitiveMsg)
}

func TestAuditingLLM_StreamComplete_LogsMetrics(t *testing.T) {
	var buf bytes.Buffer
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))

	stub := &stubLLM{name: "test-provider"}
	auditing := WithAudit(stub)
	_, _ = auditing.StreamComplete(context.Background(), CompletionRequest{})

	var entry map[string]interface{}
	require.NoError(t, json.Unmarshal(buf.Bytes(), &entry))

	assert.Equal(t, "llm_audit", entry["msg"])
	assert.Equal(t, "test-provider", entry["provider"])
	assert.Equal(t, "stream_complete", entry["operation"])
	assert.Contains(t, entry, "latency_ms")
	assert.Equal(t, "none", entry["error"])
}
