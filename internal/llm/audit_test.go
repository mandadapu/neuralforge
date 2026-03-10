package llm

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
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
	ch := make(chan StreamChunk)
	close(ch)
	return ch, s.err
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
