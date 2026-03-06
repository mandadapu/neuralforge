package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("NEURALFORGE_PORT", "9090")
	t.Setenv("NEURALFORGE_WORKERS", "3")
	t.Setenv("GITHUB_APP_ID", "12345")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "test-secret")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")

	cfg := LoadFromEnv()

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, 3, cfg.Workers)
	assert.Equal(t, int64(12345), cfg.GitHub.AppID)
	assert.Equal(t, "test-secret", cfg.GitHub.WebhookSecret)
	assert.Equal(t, "sk-ant-test", cfg.LLM.Claude.APIKey)
}

func TestLoadFromEnvDefaults(t *testing.T) {
	for _, k := range []string{"NEURALFORGE_PORT", "NEURALFORGE_WORKERS"} {
		os.Unsetenv(k)
	}

	cfg := LoadFromEnv()

	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 5, cfg.Workers)
	assert.Equal(t, "sqlite", cfg.Store.Driver)
}
