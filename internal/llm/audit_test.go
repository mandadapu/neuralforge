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
