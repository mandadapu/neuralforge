package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newClaudeTestServer(t *testing.T, handler http.HandlerFunc) (*ClaudeBackend, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := anthropic.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(srv.URL),
	)
	backend := &ClaudeBackend{
		client: client,
		model:  "claude-sonnet-4-5-20250514",
	}
	return backend, srv
}

func TestClaudeBackend_Name(t *testing.T) {
	b := &ClaudeBackend{model: "test"}
	assert.Equal(t, "claude", b.Name())
}

func TestClaudeBackend_Complete_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"id":           "msg_test123",
			"type":         "message",
			"role":         "assistant",
			"content":      []map[string]any{{"type": "text", "text": "Hello from Claude!"}},
			"model":        "claude-sonnet-4-5-20250514",
			"stop_reason":  "end_turn",
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":  100,
				"output_tokens": 50,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	backend, _ := newClaudeTestServer(t, handler)

	resp, err := backend.Complete(context.Background(), CompletionRequest{
		System: "You are a helpful assistant.",
		Messages: []Message{
			{Role: RoleUser, Content: "Say hello"},
		},
		MaxTokens:   256,
		Temperature: 0.5,
	})

	require.NoError(t, err)
	assert.Equal(t, "Hello from Claude!", resp.Content)
	assert.Equal(t, 100, resp.InputTokens)
	assert.Equal(t, 50, resp.OutputTokens)
	assert.Greater(t, resp.Cost, 0.0)
}

func TestClaudeBackend_Complete_UsesDefaultModel(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"id":           "msg_test",
			"type":         "message",
			"role":         "assistant",
			"content":      []map[string]any{{"type": "text", "text": "ok"}},
			"model":        "claude-sonnet-4-5-20250514",
			"stop_reason":  "end_turn",
			"stop_sequence": nil,
			"usage":        map[string]any{"input_tokens": 10, "output_tokens": 5},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	backend, _ := newClaudeTestServer(t, handler)

	// When req.Model is empty, backend.model is used
	resp, err := backend.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Content)
}

func TestClaudeBackend_Complete_MultipleTextBlocks(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"id":           "msg_multi",
			"type":         "message",
			"role":         "assistant",
			"content":      []map[string]any{{"type": "text", "text": "Part 1"}, {"type": "text", "text": "Part 2"}},
			"model":        "claude-sonnet-4-5-20250514",
			"stop_reason":  "end_turn",
			"stop_sequence": nil,
			"usage":        map[string]any{"input_tokens": 10, "output_tokens": 10},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	backend, _ := newClaudeTestServer(t, handler)

	resp, err := backend.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "split"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "Part 1Part 2", resp.Content)
}

func TestClaudeBackend_Complete_APIError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"type":  "error",
			"error": map[string]any{"type": "authentication_error", "message": "Invalid API key"},
		})
	})

	backend, _ := newClaudeTestServer(t, handler)

	_, err := backend.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "claude completion failed")
}

func TestClaudeBackend_StreamComplete_NotImplemented(t *testing.T) {
	b := &ClaudeBackend{model: "test"}
	_, err := b.StreamComplete(context.Background(), CompletionRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestClaudeBackend_Complete_CostCalculated(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"id":           "msg_cost",
			"type":         "message",
			"role":         "assistant",
			"content":      []map[string]any{{"type": "text", "text": "response"}},
			"model":        "claude-sonnet-4-5-20250514",
			"stop_reason":  "end_turn",
			"stop_sequence": nil,
			"usage": map[string]any{
				"input_tokens":  1000000,
				"output_tokens": 0,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	backend, _ := newClaudeTestServer(t, handler)

	resp, err := backend.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})

	require.NoError(t, err)
	// 1M input tokens at $3.00/M = $3.00
	assert.InDelta(t, 3.0, resp.Cost, 0.01)
}
