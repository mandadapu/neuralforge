package llm

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockLLM is a simple LLM implementation for testing.
type mockLLM struct {
	name string
	resp CompletionResponse
	err  error
}

func (m *mockLLM) Name() string { return m.name }

func (m *mockLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	return m.resp, m.err
}

func (m *mockLLM) StreamComplete(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
	return nil, nil
}

// testSlogHandler captures slog records for assertion.
type testSlogHandler struct {
	records []slog.Record
}

func (h *testSlogHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *testSlogHandler) Handle(_ context.Context, r slog.Record) error {
	h.records = append(h.records, r)
	return nil
}

func (h *testSlogHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *testSlogHandler) WithGroup(name string) slog.Handler       { return h }

func TestAuditingLLM_Complete(t *testing.T) {
	mock := &mockLLM{
		name: "test-provider",
		resp: CompletionResponse{
			Content:      "hello",
			Model:        "test-model",
			InputTokens:  10,
			OutputTokens: 5,
			Cost:         0.001,
		},
	}

	handler := &testSlogHandler{}
	orig := slog.Default()
	slog.SetDefault(slog.New(handler))
	t.Cleanup(func() { slog.SetDefault(orig) })

	audited := NewAudited(mock)
	require.Equal(t, "test-provider", audited.Name())

	resp, err := audited.Complete(context.Background(), CompletionRequest{Model: "test-model"})
	require.NoError(t, err)
	assert.Equal(t, mock.resp, resp)

	require.Len(t, handler.records, 1)
	rec := handler.records[0]
	assert.Equal(t, "llm_audit", rec.Message)

	attrs := map[string]any{}
	rec.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Any()
		return true
	})
	assert.Equal(t, "test-provider", attrs["provider"])
	assert.Equal(t, "test-model", attrs["model"])
	assert.Equal(t, int64(10), attrs["input_tokens"])
	assert.Equal(t, int64(5), attrs["output_tokens"])
}

func TestAuditingLLM_StreamComplete(t *testing.T) {
	mock := &mockLLM{name: "test-provider"}
	audited := NewAudited(mock)
	ch, err := audited.StreamComplete(context.Background(), CompletionRequest{})
	assert.Nil(t, ch)
	assert.NoError(t, err)
}
