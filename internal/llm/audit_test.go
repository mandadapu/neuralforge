package llm

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan StreamChunk, 2)
	ch <- StreamChunk{Content: "hello"}
	ch <- StreamChunk{Content: " world", Done: true}
	close(ch)
	return ch, nil
}

func TestAuditedLLM_Complete(t *testing.T) {
	tests := []struct {
		name    string
		inner   LLM
		wantErr bool
	}{
		{
			name:  "success",
			inner: &mockLLM{name: "mock", resp: CompletionResponse{Content: "hi", Model: "m"}},
		},
		{
			name:    "error propagated",
			inner:   &mockLLM{name: "mock", err: fmt.Errorf("fail")},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			a := NewAudited(tc.inner)
			assert.Equal(t, "mock", a.Name())
			_, err := a.Complete(context.Background(), CompletionRequest{})
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAuditedLLM_StreamComplete(t *testing.T) {
	t.Run("success emits all chunks", func(t *testing.T) {
		a := NewAudited(&mockLLM{name: "mock"})
		ch, err := a.StreamComplete(context.Background(), CompletionRequest{})
		assert.NoError(t, err)
		var chunks []StreamChunk
		for c := range ch {
			chunks = append(chunks, c)
		}
		assert.Len(t, chunks, 2)
		assert.True(t, chunks[1].Done)
	})

	t.Run("error propagated", func(t *testing.T) {
		a := NewAudited(&mockLLM{name: "mock", err: fmt.Errorf("stream fail")})
		ch, err := a.StreamComplete(context.Background(), CompletionRequest{})
		assert.Error(t, err)
		assert.Nil(t, ch)
	})
}

func TestNew(t *testing.T) {
	// Verify New returns an *AuditedLLM for both providers
	for _, provider := range []string{"openai", "claude"} {
		client := New(provider, "key", "model")
		_, ok := client.(*AuditedLLM)
		assert.True(t, ok, "expected AuditedLLM for provider %s", provider)
	}
}
