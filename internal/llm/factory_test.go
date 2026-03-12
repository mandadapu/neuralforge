package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew_ProviderRouting(t *testing.T) {
	tests := []struct {
		name         string
		provider     string
		wantInner    any
		wantErr      bool
	}{
		{
			name:      "openai provider returns AuditedLLM wrapping OpenAIBackend",
			provider:  "openai",
			wantInner: &OpenAIBackend{},
		},
		{
			name:      "claude provider returns AuditedLLM wrapping ClaudeBackend",
			provider:  "claude",
			wantInner: &ClaudeBackend{},
		},
		{
			name:      "empty provider defaults to claude",
			provider:  "",
			wantInner: &ClaudeBackend{},
		},
		{
			name:     "unknown provider returns error",
			provider: "gemini",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := New(tt.provider, "test-key", "test-model")
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.provider)
				assert.Nil(t, result)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, result)

			// Result must be an AuditedLLM.
			audited, ok := result.(*AuditedLLM)
			require.True(t, ok, "expected *AuditedLLM, got %T", result)

			// Inner must be the correct backend type.
			assert.IsType(t, tt.wantInner, audited.inner)
		})
	}
}

func TestNew_AlwaysWrapsWithAuditLayer(t *testing.T) {
	llm, err := New("claude", "key", "model")
	require.NoError(t, err)
	_, ok := llm.(*AuditedLLM)
	assert.True(t, ok, "New must always return an *AuditedLLM to ensure audit controls are present")
}

func TestNew_UnknownProviderErrorMessage(t *testing.T) {
	_, err := New("unknown-provider", "", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown llm provider")
	assert.Contains(t, err.Error(), "unknown-provider")
}
