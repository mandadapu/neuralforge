package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeployStage_Disabled(t *testing.T) {
	s := NewDeployStage(false)
	assert.Equal(t, "deploy", s.Name())

	state := &PipelineState{}
	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
}

func TestDeployStage_NotMerged(t *testing.T) {
	s := NewDeployStage(true)
	state := &PipelineState{Merged: false}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "not merged")
}

func TestDeployStage_Merged(t *testing.T) {
	s := NewDeployStage(true)
	state := &PipelineState{Merged: true}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	assert.Contains(t, result.Output, "Deploy triggered")
}
