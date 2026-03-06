package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepoConfig(t *testing.T) {
	yaml := `
neuralforge:
  trigger:
    label: "autofix"
  llm:
    provider: claude
    model: claude-sonnet-4-5-20250514
  executor:
    type: docker
  pipeline:
    architecture_review: true
    security_review: true
    code_review: true
    auto_merge: false
  limits:
    max_files_changed: 50
    timeout_minutes: 30
    budget_usd: 5.0
`
	cfg, err := ParseRepoConfig([]byte(yaml))
	require.NoError(t, err)

	assert.Equal(t, "autofix", cfg.Trigger.Label)
	assert.Equal(t, "claude", cfg.LLM.Provider)
	assert.True(t, cfg.Pipeline.ArchitectureReview)
	assert.False(t, cfg.Pipeline.AutoMerge)
	assert.Equal(t, 50, cfg.Limits.MaxFilesChanged)
	assert.InDelta(t, 5.0, cfg.Limits.BudgetUSD, 0.01)
}

func TestParseRepoConfigDefaults(t *testing.T) {
	cfg, err := ParseRepoConfig([]byte(""))
	require.NoError(t, err)

	assert.Equal(t, "neuralforge", cfg.Trigger.Label)
	assert.Equal(t, 5.0, cfg.Limits.BudgetUSD)
}
