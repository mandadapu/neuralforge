package llm

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubLLM struct {
	name        string
	resp        CompletionResponse
	err         error
	called      bool
	streamChunks []StreamChunk
}

func (s *stubLLM) Name() string { return s.name }

func (s *stubLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	s.called = true
	return s.resp, s.err
}

func (s *stubLLM) StreamComplete(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
	if s.err != nil {
		return nil, s.err
	}
	ch := make(chan StreamChunk, len(s.streamChunks))
	for _, c := range s.streamChunks {
		ch <- c
	}
	close(ch)
	return ch, nil
}

// captureHandler is a minimal slog.Handler that records all log attributes
// as "key=value" strings so tests can assert on what was (not) logged.
type captureHandler struct {
	records []string
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }
func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler      { return h }
func (h *captureHandler) WithGroup(name string) slog.Handler            { return h }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	r.Attrs(func(a slog.Attr) bool {
		h.records = append(h.records, a.Key+"="+a.Value.String())
		return true
	})
	// Also capture the message itself.
	h.records = append(h.records, "msg="+r.Message)
	return nil
}

func (h *captureHandler) contains(s string) bool {
	for _, rec := range h.records {
		if strings.Contains(rec, s) {
			return true
		}
	}
	return false
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

func TestAuditedLLM_StreamComplete_relays_chunks(t *testing.T) {
	chunks := []StreamChunk{
		{Content: "hello"},
		{Content: " world", Done: true},
	}
	stub := &stubLLM{name: "stub", streamChunks: chunks}
	audited := NewAudited(stub)

	ch, err := audited.StreamComplete(context.Background(), CompletionRequest{Model: "gpt-4"})

	require.NoError(t, err)
	require.NotNil(t, ch)

	var got []StreamChunk
	for c := range ch {
		got = append(got, c)
	}
	assert.Equal(t, chunks, got)
}

// TestAuditedLLM_Complete_noPIILogged verifies that message content, system
// prompt text, and response content are never written to the audit log,
// preventing accidental exposure of PII, PHI, or PAN.
func TestAuditedLLM_Complete_noPIILogged(t *testing.T) {
	h := &captureHandler{}
	orig := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(orig)

	sensitiveContent := "SSN: 123-45-6789 card: 4111111111111111"
	stub := &stubLLM{
		name: "stub",
		resp: CompletionResponse{
			Content:      sensitiveContent,
			Model:        "gpt-4",
			InputTokens:  5,
			OutputTokens: 10,
		},
	}
	audited := NewAudited(stub)

	req := CompletionRequest{
		System:   sensitiveContent,
		Messages: []Message{{Role: RoleUser, Content: sensitiveContent}},
		Model:    "gpt-4",
	}
	_, err := audited.Complete(context.Background(), req)
	require.NoError(t, err)

	assert.False(t, h.contains(sensitiveContent),
		"audit log must not contain PII/PHI/PAN content; got records: %v", h.records)
}

// TestAuditedLLM_StreamComplete_noPIILogged verifies that streamed content is
// not written to the audit log.
func TestAuditedLLM_StreamComplete_noPIILogged(t *testing.T) {
	h := &captureHandler{}
	orig := slog.Default()
	slog.SetDefault(slog.New(h))
	defer slog.SetDefault(orig)

	sensitiveContent := "secret PII data"
	stub := &stubLLM{
		name: "stub",
		streamChunks: []StreamChunk{
			{Content: sensitiveContent},
			{Content: " more", Done: true},
		},
	}
	audited := NewAudited(stub)

	ch, err := audited.StreamComplete(context.Background(), CompletionRequest{Model: "gpt-4"})
	require.NoError(t, err)
	for range ch {
	}

	assert.False(t, h.contains(sensitiveContent),
		"audit log must not contain streamed content; got records: %v", h.records)
}
