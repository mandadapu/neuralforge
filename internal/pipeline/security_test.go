package pipeline

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mandadapu/neuralforge/internal/llm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityStage_Name(t *testing.T) {
	s := NewSecurityStage(&mockLLM{})
	assert.Equal(t, "security", s.Name())
}

func TestSecurityStage_SkipsWhenNoPlan(t *testing.T) {
	s := NewSecurityStage(&mockLLM{})
	state := &PipelineState{Plan: ""}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "no plan")
}

func TestSecurityStage_SetsSecurityNotesFromLLMResponse(t *testing.T) {
	ml := &mockLLM{
		response: llm.CompletionResponse{
			Content: "No critical security issues found.",
			Cost:    0.02,
		},
	}

	s := NewSecurityStage(ml)
	state := &PipelineState{
		Plan: "Implement new endpoint",
		Repo: RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, "No critical security issues found.", state.SecurityNotes)
	assert.InDelta(t, 0.02, state.Cost, 0.0001)
}

func TestSecurityStage_IncludesPlanAndRepoInPrompt(t *testing.T) {
	ml := &mockLLM{
		response: llm.CompletionResponse{Content: "ok"},
	}

	s := NewSecurityStage(ml)
	state := &PipelineState{
		Plan: "My implementation plan",
		Repo: RepoContext{FullName: "myorg/myrepo"},
	}

	_, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	prompt := ml.lastReq.Messages[0].Content
	assert.True(t, strings.Contains(prompt, "My implementation plan"))
	assert.True(t, strings.Contains(prompt, "myorg/myrepo"))
}

func TestSecurityStage_LLMErrorPropagates(t *testing.T) {
	ml := &mockLLM{
		err: errors.New("LLM failed"),
	}

	s := NewSecurityStage(ml)
	state := &PipelineState{
		Plan: "some plan",
		Repo: RepoContext{FullName: "o/r"},
	}

	_, err := s.Run(context.Background(), state)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "security llm call")
}
