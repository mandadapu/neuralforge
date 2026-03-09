package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newOpenAITestServer(t *testing.T, handler http.HandlerFunc) (*OpenAIBackend, *httptest.Server) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client := openai.NewClient(
		option.WithAPIKey("test-key"),
		option.WithBaseURL(srv.URL),
	)
	backend := &OpenAIBackend{
		client: client,
		model:  "gpt-4o",
	}
	return backend, srv
}

func TestOpenAIBackend_Name(t *testing.T) {
	b := &OpenAIBackend{model: "test"}
	assert.Equal(t, "openai", b.Name())
}

func TestOpenAIBackend_Complete_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"id":      "chatcmpl-test123",
			"object":  "chat.completion",
			"created": 1677858242,
			"model":   "gpt-4o",
			"choices": []map[string]any{
				{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "Hello from OpenAI!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     100,
				"completion_tokens": 50,
				"total_tokens":      150,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	backend, _ := newOpenAITestServer(t, handler)

	resp, err := backend.Complete(context.Background(), CompletionRequest{
		System: "You are a helpful assistant.",
		Messages: []Message{
			{Role: RoleUser, Content: "Say hello"},
		},
		MaxTokens:   256,
		Temperature: 0.7,
	})

	require.NoError(t, err)
	assert.Equal(t, "Hello from OpenAI!", resp.Content)
	assert.Equal(t, 100, resp.InputTokens)
	assert.Equal(t, 50, resp.OutputTokens)
	assert.Greater(t, resp.Cost, 0.0)
}

func TestOpenAIBackend_Complete_EmptyChoices(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"id":      "chatcmpl-empty",
			"object":  "chat.completion",
			"created": 1677858242,
			"model":   "gpt-4o",
			"choices": []map[string]any{},
			"usage": map[string]any{
				"prompt_tokens":     10,
				"completion_tokens": 0,
				"total_tokens":      10,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	backend, _ := newOpenAITestServer(t, handler)

	resp, err := backend.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "", resp.Content)
}

func TestOpenAIBackend_Complete_UsesDefaultModel(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"id":      "chatcmpl-default",
			"object":  "chat.completion",
			"created": 1677858242,
			"model":   "gpt-4o",
			"choices": []map[string]any{
				{
					"index":   0,
					"message": map[string]any{"role": "assistant", "content": "ok"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     5,
				"completion_tokens": 2,
				"total_tokens":      7,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	backend, _ := newOpenAITestServer(t, handler)

	// req.Model is empty, so backend.model ("gpt-4o") is used
	resp, err := backend.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})

	require.NoError(t, err)
	assert.Equal(t, "ok", resp.Content)
}

func TestOpenAIBackend_Complete_APIError(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error": map[string]any{
				"message": "Invalid API key",
				"type":    "invalid_request_error",
			},
		})
	})

	backend, _ := newOpenAITestServer(t, handler)

	_, err := backend.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "openai completion failed")
}

func TestOpenAIBackend_StreamComplete_NotImplemented(t *testing.T) {
	b := &OpenAIBackend{model: "test"}
	_, err := b.StreamComplete(context.Background(), CompletionRequest{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestOpenAIBackend_Complete_CostCalculated(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		resp := map[string]any{
			"id":      "chatcmpl-cost",
			"object":  "chat.completion",
			"created": 1677858242,
			"model":   "gpt-4o",
			"choices": []map[string]any{
				{
					"index":   0,
					"message": map[string]any{"role": "assistant", "content": "response"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     1000000,
				"completion_tokens": 0,
				"total_tokens":      1000000,
			},
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	backend, _ := newOpenAITestServer(t, handler)

	resp, err := backend.Complete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})

	require.NoError(t, err)
	// 1M prompt tokens for gpt-4o at $2.50/M = $2.50
	assert.InDelta(t, 2.50, resp.Cost, 0.01)
}
