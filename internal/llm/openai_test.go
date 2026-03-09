package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIBackendMaxTokens(t *testing.T) {
	tests := []struct {
		name              string
		requestMaxTokens  int
		expectedMaxTokens float64
	}{
		{
			name:              "zero MaxTokens defaults to 4096",
			requestMaxTokens:  0,
			expectedMaxTokens: 4096,
		},
		{
			name:              "explicit MaxTokens preserved",
			requestMaxTokens:  2048,
			expectedMaxTokens: 2048,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var capturedBody []byte

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedBody, _ = io.ReadAll(r.Body)
				resp := map[string]interface{}{
					"id":      "chatcmpl-test",
					"object":  "chat.completion",
					"created": 1234567890,
					"model":   "gpt-4",
					"choices": []map[string]interface{}{
						{
							"index": 0,
							"message": map[string]interface{}{
								"role":    "assistant",
								"content": "test response",
							},
							"finish_reason": "stop",
						},
					},
					"usage": map[string]interface{}{
						"prompt_tokens":     10,
						"completion_tokens": 5,
						"total_tokens":      15,
					},
				}
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(resp)
			}))
			defer server.Close()

			backend := &OpenAIBackend{
				client: openai.NewClient(
					option.WithAPIKey("test-key"),
					option.WithBaseURL(server.URL),
				),
				model: "gpt-4",
			}

			req := CompletionRequest{
				Messages:  []Message{{Role: RoleUser, Content: "hello"}},
				MaxTokens: tt.requestMaxTokens,
			}

			_, err := backend.Complete(context.Background(), req)
			require.NoError(t, err)

			var body map[string]interface{}
			err = json.Unmarshal(capturedBody, &body)
			require.NoError(t, err)

			actualMaxTokens, ok := body["max_completion_tokens"]
			require.True(t, ok, "max_completion_tokens must be present in request body")
			assert.Equal(t, tt.expectedMaxTokens, actualMaxTokens)
		})
	}
}
