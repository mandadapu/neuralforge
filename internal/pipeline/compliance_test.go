package pipeline

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestComplianceStageName(t *testing.T) {
	s := NewComplianceStage(2000, 50)
	assert.Equal(t, "compliance", s.Name())
}

func TestComplianceStageNoLocalPath(t *testing.T) {
	s := NewComplianceStage(2000, 50)
	state := &PipelineState{Repo: RepoContext{LocalPath: ""}}

	result, err := s.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "no local path available")
}

func TestComplianceStageZeroLimitsSkipChecks(t *testing.T) {
	// Zero limits mean no limit enforcement — valid state for the struct.
	s := NewComplianceStage(0, 0)
	assert.Equal(t, 0, s.maxDiffLines)
	assert.Equal(t, 0, s.maxFilesChanged)
}
