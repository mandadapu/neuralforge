package llm

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockLLM struct {
	name     string
	resp     CompletionResponse
	err      error
	called   bool
	lastReq  CompletionRequest
}

func (m *mockLLM) Name() string { return m.name }

func (m *mockLLM) Complete(_ context.Context, req CompletionRequest) (CompletionResponse, error) {
	m.called = true
	m.lastReq = req
	return m.resp, m.err
}

func (m *mockLLM) StreamComplete(_ context.Context, _ CompletionRequest) (<-chan StreamChunk, error) {
	return nil, fmt.Errorf("not implemented")
}

func TestAuditedBackend(t *testing.T) {
	tests := []struct {
		name      string
		mockName  string
		mockResp  CompletionResponse
		mockErr   error
		req       CompletionRequest
		wantErr   bool
	}{
		{
			name:     "delegates to inner backend and returns response",
			mockName: "openai",
			mockResp: CompletionResponse{
				Content:      "hello",
				Model:        "gpt-4",
				InputTokens:  10,
				OutputTokens: 5,
				Cost:         0.001,
			},
			req: CompletionRequest{
				Model:    "gpt-4",
				Messages: []Message{{Role: RoleUser, Content: "hi"}},
			},
			wantErr: false,
		},
		{
			name:     "propagates errors from inner backend",
			mockName: "claude",
			mockErr:  fmt.Errorf("api error"),
			req: CompletionRequest{
				Model: "claude-3",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockLLM{
				name: tt.mockName,
				resp: tt.mockResp,
				err:  tt.mockErr,
			}

			audited := NewAudited(mock)

			assert.Equal(t, tt.mockName, audited.Name())

			resp, err := audited.Complete(context.Background(), tt.req)

			assert.True(t, mock.called, "inner backend should be called")
			assert.Equal(t, tt.req, mock.lastReq, "request should be passed through unchanged")

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.mockResp, resp, "response should be passed through unchanged")
			}
		})
	}
}
