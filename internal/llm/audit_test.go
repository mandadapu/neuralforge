package llm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type stubLLM struct {
	name       string
	resp       CompletionResponse
	err        error
	streamCh   chan StreamChunk
	streamErr  error
}

func (s *stubLLM) Name() string { return s.name }

func (s *stubLLM) Complete(_ context.Context, _ CompletionRequest) (CompletionResponse, error) {
	return s.resp, s.err
}

func (s *stubLLM) StreamComplete(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
	if s.streamErr != nil {
		return nil, s.streamErr
	}
	if s.streamCh != nil {
		return s.streamCh, nil
	}
	ch := make(chan StreamChunk)
	close(ch)
	return ch, nil
}

func TestAuditedLLM_Name(t *testing.T) {
	inner := &stubLLM{name: "test-provider"}
	audited := NewAuditedLLM(inner)
	assert.Equal(t, "test-provider", audited.Name())
}

func TestAuditedLLM_Complete(t *testing.T) {
	tests := []struct {
		name    string
		inner   *stubLLM
		wantErr bool
	}{
		{
			name: "success delegates response",
			inner: &stubLLM{
				name: "openai",
				resp: CompletionResponse{
					Content:      "hello",
					Model:        "gpt-4",
					InputTokens:  10,
					OutputTokens: 5,
					Cost:         0.001,
				},
			},
		},
		{
			name: "error propagated",
			inner: &stubLLM{
				name: "openai",
				err:  errors.New("backend error"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audited := NewAuditedLLM(tt.inner)
			resp, err := audited.Complete(context.Background(), CompletionRequest{})
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.inner.resp, resp)
			}
		})
	}
}

func TestSanitizeError(t *testing.T) {
	assert.Nil(t, sanitizeError(nil))
	assert.Equal(t, "error_occurred", sanitizeError(errors.New("some PII data")))
}

func TestAuditedLLM_StreamComplete(t *testing.T) {
	t.Run("init error is returned and logged", func(t *testing.T) {
		inner := &stubLLM{name: "openai", streamErr: errors.New("dial failed")}
		audited := NewAuditedLLM(inner)
		ch, err := audited.StreamComplete(context.Background(), CompletionRequest{})
		assert.Error(t, err)
		assert.Nil(t, ch)
	})

	t.Run("chunks forwarded and channel closed", func(t *testing.T) {
		innerCh := make(chan StreamChunk, 2)
		innerCh <- StreamChunk{Content: "hello"}
		innerCh <- StreamChunk{Content: " world", Done: true}
		close(innerCh)

		inner := &stubLLM{name: "openai", streamCh: innerCh}
		audited := NewAuditedLLM(inner)
		ch, err := audited.StreamComplete(context.Background(), CompletionRequest{})
		assert.NoError(t, err)

		var chunks []StreamChunk
		for c := range ch {
			chunks = append(chunks, c)
		}
		assert.Len(t, chunks, 2)
		assert.Equal(t, "hello", chunks[0].Content)
		assert.Equal(t, " world", chunks[1].Content)
	})

	t.Run("stream chunk error is captured in audit log", func(t *testing.T) {
		innerCh := make(chan StreamChunk, 1)
		innerCh <- StreamChunk{Error: errors.New("stream error")}
		close(innerCh)

		inner := &stubLLM{name: "openai", streamCh: innerCh}
		audited := NewAuditedLLM(inner)
		ch, err := audited.StreamComplete(context.Background(), CompletionRequest{})
		assert.NoError(t, err)

		// Drain the channel; the goroutine logs the error on close.
		for range ch {
		}
	})
}
