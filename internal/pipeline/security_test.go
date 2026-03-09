package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityStage_Name(t *testing.T) {
	s := NewSecurityStage(&mockLLM{})
	assert.Equal(t, "security", s.Name())
}

func TestSecurityStage_SkipEmptyPlan(t *testing.T) {
	s := NewSecurityStage(&mockLLM{})
	state := &PipelineState{Plan: ""}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Equal(t, "", state.SecurityNotes)
}

func TestSecurityStage_Success(t *testing.T) {
	llmMock := &mockLLM{content: "No critical issues found.", cost: 0.05}
	s := NewSecurityStage(llmMock)
	state := &PipelineState{
		Plan: "Step 1: Add a new endpoint",
		Repo: RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, "No critical issues found.", state.SecurityNotes)
	assert.InDelta(t, 0.05, state.Cost, 0.001)
	assert.Contains(t, result.Output, "Security review")
}

func TestSecurityStage_LLMError(t *testing.T) {
	llmMock := &mockLLM{err: assert.AnError}
	s := NewSecurityStage(llmMock)
	state := &PipelineState{Plan: "do something"}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "security llm call")
}
