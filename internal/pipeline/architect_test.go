package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchitectStage_Name(t *testing.T) {
	s := NewArchitectStage(&mockLLM{})
	assert.Equal(t, "architect", s.Name())
}

func TestArchitectStage_Success(t *testing.T) {
	llmMock := &mockLLM{content: "Step 1: modify foo.go", cost: 0.10}
	s := NewArchitectStage(llmMock)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1, Title: "Fix bug", Body: "It's broken"},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, "Step 1: modify foo.go", state.Plan)
	assert.InDelta(t, 0.10, state.Cost, 0.001)
	assert.Contains(t, result.Output, "plan")
}

func TestArchitectStage_CostAccumulates(t *testing.T) {
	llmMock := &mockLLM{content: "plan", cost: 0.20}
	s := NewArchitectStage(llmMock)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1, Title: "T"},
		Cost:  0.05, // pre-existing cost
	}

	_, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.InDelta(t, 0.25, state.Cost, 0.001)
}

func TestArchitectStage_LLMError(t *testing.T) {
	llmMock := &mockLLM{err: assert.AnError}
	s := NewArchitectStage(llmMock)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1, Title: "Test"},
	}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "architect llm call")
}
