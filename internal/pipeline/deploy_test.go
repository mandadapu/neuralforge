package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeployStageName(t *testing.T) {
	s := NewDeployStage(false)
	assert.Equal(t, "deploy", s.Name())
}

func TestDeployStageDisabled(t *testing.T) {
	s := NewDeployStage(false)
	state := &PipelineState{Merged: true}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "disabled")
}

func TestDeployStageNotMerged(t *testing.T) {
	s := NewDeployStage(true)
	state := &PipelineState{Merged: false}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "not merged")
}

func TestDeployStageSuccess(t *testing.T) {
	s := NewDeployStage(true)
	state := &PipelineState{Merged: true}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Contains(t, result.Output, "Deploy triggered")
}
