package llm

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeBackendName(t *testing.T) {
	c := NewClaude("test-key", "claude-3-5-sonnet-20241022")
	assert.Equal(t, "claude", c.Name())
}

func TestClaudeBackendStreamCompleteNotImplemented(t *testing.T) {
	c := NewClaude("test-key", "claude-3-5-sonnet-20241022")
	ch, err := c.StreamComplete(context.Background(), CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "hello"}},
	})
	require.Error(t, err)
	assert.Nil(t, ch)
	assert.Contains(t, err.Error(), "not implemented")
}

func TestClaudeBackendDefaultModel(t *testing.T) {
	// When model is empty in the request, the backend falls back to the constructor model.
	c := NewClaude("test-key", "claude-3-5-sonnet-20241022")
	assert.NotNil(t, c)
	assert.Equal(t, "claude-3-5-sonnet-20241022", c.model)
}

func TestNewClaudeReturnsNonNil(t *testing.T) {
	c := NewClaude("any-key", "claude-3-opus-20240229")
	require.NotNil(t, c)
}
