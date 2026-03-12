package llm

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockLLM struct {
	name     string
	resp     CompletionResponse
	err      error
	calls    int
}

func (m *mockLLM) Name() string { return m.name }

func (m *mockLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	m.calls++
	return m.resp, m.err
}

func (m *mockLLM) StreamComplete(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
	ch := make(chan StreamChunk)
	close(ch)
	return ch, m.err
}

func newTestLogger(buf *bytes.Buffer) *slog.Logger {
	return slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

func TestAuditedLLM_Name(t *testing.T) {
	mock := &mockLLM{name: "mock"}
	audited := NewAuditedLLM(mock, nil)
	assert.Equal(t, "audited/mock", audited.Name())
}

func TestAuditedLLM_Complete(t *testing.T) {
	tests := []struct {
		name        string
		resp        CompletionResponse
		err         error
		wantLogKeys []string
		wantErr     bool
	}{
		{
			name: "success",
			resp: CompletionResponse{
				Content:      "hello",
				Model:        "gpt-4",
				InputTokens:  10,
				OutputTokens: 5,
				Cost:         0.001,
			},
			wantLogKeys: []string{"llm.call.start", "llm.call.done", "provider", "model", "cost_usd", "duration_ms"},
		},
		{
			name:        "error",
			err:         errors.New("api failure"),
			wantLogKeys: []string{"llm.call.start", "llm.call.error", "provider", "model", "duration_ms", "error"},
			wantErr:     true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			logger := newTestLogger(&buf)
			mock := &mockLLM{name: "testprovider", resp: tc.resp, err: tc.err}
			audited := NewAuditedLLM(mock, logger)

			req := CompletionRequest{Model: "gpt-4", Messages: []Message{{Role: RoleUser, Content: "hi"}}}
			resp, err := audited.Complete(context.Background(), req)

			if tc.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.resp, resp)
			}
			assert.Equal(t, 1, mock.calls)

			logOutput := buf.String()
			for _, key := range tc.wantLogKeys {
				assert.True(t, strings.Contains(logOutput, key), "expected log key %q in output: %s", key, logOutput)
			}
		})
	}
}

func TestAuditedLLM_NilLogger(t *testing.T) {
	mock := &mockLLM{name: "mock", resp: CompletionResponse{Content: "ok"}}
	audited := NewAuditedLLM(mock, nil)
	// Should not panic with nil logger.
	_, err := audited.Complete(context.Background(), CompletionRequest{})
	assert.NoError(t, err)
}

func TestAuditedLLM_StreamComplete(t *testing.T) {
	t.Run("success logs start and done", func(t *testing.T) {
		var buf bytes.Buffer
		logger := newTestLogger(&buf)
		mock := &mockLLM{name: "streamprovider"}
		audited := NewAuditedLLM(mock, logger)

		ch, err := audited.StreamComplete(context.Background(), CompletionRequest{Model: "gpt-4"})
		require.NoError(t, err)
		// Drain the channel (mock closes it immediately with Done chunk).
		for range ch {
		}

		logOutput := buf.String()
		for _, key := range []string{"llm.stream.start", "llm.stream.done", "provider", "model", "duration_ms", "input_tokens", "output_tokens", "cost_usd"} {
			assert.True(t, strings.Contains(logOutput, key), "expected log key %q in output: %s", key, logOutput)
		}
	})

	t.Run("init error logs stream.error", func(t *testing.T) {
		var buf bytes.Buffer
		logger := newTestLogger(&buf)
		mock := &mockLLM{name: "streamprovider", err: errors.New("dial tcp: connection refused")}
		audited := NewAuditedLLM(mock, logger)

		_, err := audited.StreamComplete(context.Background(), CompletionRequest{Model: "gpt-4"})
		require.Error(t, err)

		logOutput := buf.String()
		for _, key := range []string{"llm.stream.start", "llm.stream.error", "duration_ms"} {
			assert.True(t, strings.Contains(logOutput, key), "expected log key %q in output: %s", key, logOutput)
		}
		// Full error message should be truncated / sanitized, not absent entirely.
		assert.True(t, strings.Contains(logOutput, "error"), "expected error key in output")
	})
}

func TestSanitizeError(t *testing.T) {
	assert.Equal(t, "", sanitizeError(nil))

	short := errors.New("short error")
	out := sanitizeError(short)
	// Only the type should be logged — message content must not appear in audit logs.
	assert.NotContains(t, out, "short error", "error message content must not appear in sanitized output")
	assert.NotEmpty(t, out, "sanitized output must not be empty for non-nil error")

	// Long errors should also only log the type, never the full message.
	longMsg := strings.Repeat("x", 200)
	long := errors.New(longMsg)
	out = sanitizeError(long)
	assert.NotContains(t, out, longMsg, "error message must not appear in sanitized output")
	assert.NotEmpty(t, out)
}

func TestAuditedLLM_ErrorSanitized(t *testing.T) {
	// Verify that sensitive data in an error is truncated in logs but full error
	// is returned to the caller.
	var buf bytes.Buffer
	logger := newTestLogger(&buf)
	sensitiveErr := errors.New("api_key=sk-secret123 token=eyJhb user_id=42 " + strings.Repeat("sensitive", 20))
	mock := &mockLLM{name: "prov", err: sensitiveErr}
	audited := NewAuditedLLM(mock, logger)

	_, err := audited.Complete(context.Background(), CompletionRequest{})
	require.ErrorIs(t, err, sensitiveErr) // full error returned to caller

	logOutput := buf.String()
	// Log line should not contain the full repeated "sensitive" blob.
	assert.False(t, strings.Contains(logOutput, strings.Repeat("sensitive", 20)), "full sensitive error must not appear in logs")
}
