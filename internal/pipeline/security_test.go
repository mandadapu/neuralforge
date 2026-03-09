package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/mandadapu/neuralforge/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityStageName(t *testing.T) {
	s := NewSecurityStage(&mockLLM{})
	assert.Equal(t, "security", s.Name())
}

func TestSecurityStageNoPlan(t *testing.T) {
	s := NewSecurityStage(&mockLLM{})
	state := &PipelineState{Plan: ""}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "no plan to review")
}

func TestSecurityStageSuccess(t *testing.T) {
	mockL := &mockLLM{
		resp: llm.CompletionResponse{Content: "No critical security issues found.", Cost: 0.01},
	}
	s := NewSecurityStage(mockL)
	state := &PipelineState{
		Plan: "step 1: add function\nstep 2: update tests",
		Repo: RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, "No critical security issues found.", state.SecurityNotes)
	assert.InDelta(t, 0.01, state.Cost, 1e-9)
}

func TestSecurityStageLLMError(t *testing.T) {
	mockL := &mockLLM{err: errors.New("llm timeout")}
	s := NewSecurityStage(mockL)
	state := &PipelineState{Plan: "do something potentially dangerous"}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "llm timeout")
}

func TestSecurityStageCostAccumulated(t *testing.T) {
	mockL := &mockLLM{
		resp: llm.CompletionResponse{Content: "Looks safe.", Cost: 0.03},
	}
	s := NewSecurityStage(mockL)
	state := &PipelineState{
		Plan: "update config",
		Cost: 0.07,
	}

	_, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.InDelta(t, 0.10, state.Cost, 1e-9)
}
