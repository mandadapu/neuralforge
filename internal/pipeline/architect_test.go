package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/mandadapu/neuralforge/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchitectStageName(t *testing.T) {
	s := NewArchitectStage(&mockLLM{})
	assert.Equal(t, "architect", s.Name())
}

func TestArchitectStageSuccess(t *testing.T) {
	mock := &mockLLM{
		resp: llm.CompletionResponse{Content: "step 1: do this\nstep 2: do that", Cost: 0.01},
	}
	s := NewArchitectStage(mock)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1, Title: "Test issue", Body: "Fix the bug"},
		Repo:  RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, "step 1: do this\nstep 2: do that", state.Plan)
	assert.InDelta(t, 0.01, state.Cost, 1e-9)
}

func TestArchitectStageLLMError(t *testing.T) {
	mock := &mockLLM{err: errors.New("llm unavailable")}
	s := NewArchitectStage(mock)
	state := &PipelineState{Issue: GitHubIssue{Number: 1}}

	_, err := s.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "llm unavailable")
}

func TestArchitectStageEmptyResponse(t *testing.T) {
	mock := &mockLLM{resp: llm.CompletionResponse{Content: ""}}
	s := NewArchitectStage(mock)
	state := &PipelineState{Issue: GitHubIssue{Number: 1}}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, "", state.Plan)
}

func TestArchitectStageCostAccumulated(t *testing.T) {
	mock := &mockLLM{
		resp: llm.CompletionResponse{Content: "plan", Cost: 0.05},
	}
	s := NewArchitectStage(mock)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1},
		Cost:  0.10, // pre-existing cost
	}

	_, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.InDelta(t, 0.15, state.Cost, 1e-9)
}
