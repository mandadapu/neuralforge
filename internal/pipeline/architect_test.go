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

func TestArchitectStage_Name(t *testing.T) {
	s := NewArchitectStage(&mockLLM{})
	assert.Equal(t, "architect", s.Name())
}

func TestArchitectStage_SetsPlanFromLLMResponse(t *testing.T) {
	ml := &mockLLM{
		response: llm.CompletionResponse{
			Content: "Step 1: modify foo.go\nStep 2: add tests",
			Cost:    0.05,
		},
	}

	s := NewArchitectStage(ml)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 42, Title: "Add feature X", Body: "We need feature X"},
		Repo:  RepoContext{FullName: "owner/repo"},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Equal(t, "Step 1: modify foo.go\nStep 2: add tests", state.Plan)
	assert.InDelta(t, 0.05, state.Cost, 0.0001)
}

func TestArchitectStage_IncludesIssueInPrompt(t *testing.T) {
	ml := &mockLLM{
		response: llm.CompletionResponse{Content: "plan"},
	}

	s := NewArchitectStage(ml)
	state := &PipelineState{
		Issue: GitHubIssue{
			Number: 10,
			Title:  "My Issue Title",
			Body:   "My issue body text",
		},
		Repo:   RepoContext{FullName: "owner/repo"},
		Memory: "some codebase context",
	}

	_, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.True(t, strings.Contains(ml.lastReq.Messages[0].Content, "My Issue Title"))
	assert.True(t, strings.Contains(ml.lastReq.Messages[0].Content, "My issue body text"))
	assert.NotEmpty(t, ml.lastReq.System)
}

func TestArchitectStage_LLMErrorPropagates(t *testing.T) {
	ml := &mockLLM{
		err: errors.New("LLM unavailable"),
	}

	s := NewArchitectStage(ml)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1, Title: "Test"},
	}

	_, err := s.Run(context.Background(), state)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "architect llm call")
}

func TestArchitectStage_AccumulatesCost(t *testing.T) {
	ml := &mockLLM{
		response: llm.CompletionResponse{Content: "plan", Cost: 0.10},
	}

	s := NewArchitectStage(ml)
	state := &PipelineState{
		Issue: GitHubIssue{Number: 1, Title: "Test"},
		Cost:  0.50, // pre-existing cost
	}

	_, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.InDelta(t, 0.60, state.Cost, 0.0001)
}
