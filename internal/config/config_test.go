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

func TestLoadFromEnvKubernetes(t *testing.T) {
	t.Setenv("NEURALFORGE_EXECUTOR", "kubernetes")
	t.Setenv("NEURALFORGE_K8S_NAMESPACE", "forge-ns")
	t.Setenv("NEURALFORGE_K8S_IMAGE", "my-image:v1")
	t.Setenv("NEURALFORGE_K8S_SECRET", "my-llm-keys")
	t.Setenv("NEURALFORGE_K8S_GIT_SECRET", "my-git-token")
	t.Setenv("NEURALFORGE_K8S_CPU", "4")
	t.Setenv("NEURALFORGE_K8S_MEMORY", "8Gi")

	cfg := LoadFromEnv()

	assert.Equal(t, "kubernetes", cfg.Executor.DefaultType)
	assert.Equal(t, "forge-ns", cfg.Executor.Kubernetes.Namespace)
	assert.Equal(t, "my-image:v1", cfg.Executor.Kubernetes.Image)
	assert.Equal(t, "my-llm-keys", cfg.Executor.Kubernetes.SecretName)
	assert.Equal(t, "my-git-token", cfg.Executor.Kubernetes.GitSecretName)
	assert.Equal(t, "4", cfg.Executor.Kubernetes.CPU)
	assert.Equal(t, "8Gi", cfg.Executor.Kubernetes.Memory)
}

func TestLoadFromEnvKubernetesDefaults(t *testing.T) {
	for _, k := range []string{"NEURALFORGE_K8S_NAMESPACE", "NEURALFORGE_K8S_IMAGE", "NEURALFORGE_K8S_SECRET", "NEURALFORGE_K8S_GIT_SECRET", "NEURALFORGE_K8S_CPU", "NEURALFORGE_K8S_MEMORY"} {
		os.Unsetenv(k)
	}

	cfg := LoadFromEnv()

	assert.Equal(t, "neuralforge", cfg.Executor.Kubernetes.Namespace)
	assert.Equal(t, "ghcr.io/neuralforge/claude-executor:latest", cfg.Executor.Kubernetes.Image)
	assert.Equal(t, "neuralforge-llm-keys", cfg.Executor.Kubernetes.SecretName)
	assert.Equal(t, "neuralforge-git-token", cfg.Executor.Kubernetes.GitSecretName)
	assert.Equal(t, "2", cfg.Executor.Kubernetes.CPU)
	assert.Equal(t, "4Gi", cfg.Executor.Kubernetes.Memory)
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
