package llm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenAIBackendName(t *testing.T) {
	o := NewOpenAI("test-key", "gpt-4o")
	assert.Equal(t, "openai", o.Name())
}

func TestOpenAIBackendStreamCompleteNotImplemented(t *testing.T) {
	o := NewOpenAI("test-key", "gpt-4o")
	ch, err := o.StreamComplete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	require.Error(t, err)
	assert.Nil(t, ch)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestOpenAIBackendDefaultModel(t *testing.T) {
	o := NewOpenAI("test-key", "gpt-4o-mini")
	require.NotNil(t, o)
	assert.Equal(t, "gpt-4o-mini", o.model)
}

func TestNewOpenAIReturnsNonNil(t *testing.T) {
	o := NewOpenAI("any-key", "gpt-4o")
	require.NotNil(t, o)
}
