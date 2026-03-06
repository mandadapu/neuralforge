package config

import (
	"os"
	"testing"
	"time"

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

func TestValidate_ValidConfig(t *testing.T) {
	cfg := validConfig()
	assert.NoError(t, cfg.Validate())
}

func TestValidate_Errors(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		errMsg string
	}{
		{"zero port", func(c *Config) { c.Server.Port = 0 }, "invalid server port"},
		{"port too high", func(c *Config) { c.Server.Port = 70000 }, "invalid server port"},
		{"zero workers", func(c *Config) { c.Workers = 0 }, "invalid workers"},
		{"missing app ID", func(c *Config) { c.GitHub.AppID = 0 }, "GITHUB_APP_ID is required"},
		{"missing private key", func(c *Config) { c.GitHub.PrivateKeyPath = "" }, "GITHUB_PRIVATE_KEY_PATH is required"},
		{"missing webhook secret", func(c *Config) { c.GitHub.WebhookSecret = "" }, "GITHUB_WEBHOOK_SECRET is required"},
		{"missing claude key", func(c *Config) { c.LLM.Claude.APIKey = "" }, "ANTHROPIC_API_KEY is required"},
		{"missing openai key", func(c *Config) {
			c.LLM.DefaultProvider = "openai"
			c.LLM.OpenAI.APIKey = ""
		}, "OPENAI_API_KEY is required"},
		{"bad provider", func(c *Config) { c.LLM.DefaultProvider = "gemini" }, "unknown LLM provider"},
		{"bad executor", func(c *Config) { c.Executor.DefaultType = "podman" }, "unknown executor type"},
		{"zero docker timeout", func(c *Config) { c.Executor.Docker.Timeout = 0 }, "docker timeout"},
		{"missing docker image", func(c *Config) { c.Executor.Docker.Image = "" }, "docker image is required"},
		{"zero k8s timeout", func(c *Config) {
			c.Executor.DefaultType = "kubernetes"
			c.Executor.Kubernetes = KubernetesConfig{
				Image: "img:latest", Namespace: "default", Timeout: 0,
			}
		}, "kubernetes timeout"},
		{"missing k8s image", func(c *Config) {
			c.Executor.DefaultType = "kubernetes"
			c.Executor.Kubernetes = KubernetesConfig{
				Image: "", Namespace: "default", Timeout: 30 * time.Minute,
			}
		}, "kubernetes image is required"},
		{"missing k8s namespace", func(c *Config) {
			c.Executor.DefaultType = "kubernetes"
			c.Executor.Kubernetes = KubernetesConfig{
				Image: "img:latest", Namespace: "", Timeout: 30 * time.Minute,
			}
		}, "kubernetes namespace is required"},
		{"missing store driver", func(c *Config) { c.Store.Driver = "" }, "store driver is required"},
		{"missing store DSN", func(c *Config) { c.Store.DSN = "" }, "store DSN is required"},
		{"zero refresh days", func(c *Config) { c.Context.RefreshDays = 0 }, "invalid context refresh days"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(&cfg)
			err := cfg.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.errMsg)
		})
	}
}

func TestValidate_MultipleErrors(t *testing.T) {
	cfg := Config{} // zero-value config should trigger multiple errors
	err := cfg.Validate()
	assert.Error(t, err)
	msg := err.Error()
	assert.Contains(t, msg, "config validation failed:")
	assert.Contains(t, msg, "invalid server port")
	assert.Contains(t, msg, "invalid workers")
	assert.Contains(t, msg, "GITHUB_APP_ID is required")
	assert.Contains(t, msg, "store driver is required")
}

func validConfig() Config {
	return Config{
		Server:  ServerConfig{Port: 8080, Host: "0.0.0.0"},
		Workers: 5,
		GitHub: GitHubConfig{
			AppID: 12345, PrivateKeyPath: "/path/to/key.pem", WebhookSecret: "secret",
		},
		LLM: LLMConfig{
			DefaultProvider: "claude",
			Claude:          ProviderConfig{APIKey: "sk-ant-test", Model: "claude-sonnet-4-5-20250514"},
		},
		Executor: ExecutorConfig{
			DefaultType: "docker",
			Docker:      DockerConfig{Image: "img:latest", Timeout: 30 * time.Minute},
		},
		Store:   StoreConfig{Driver: "sqlite", DSN: "neuralforge.db"},
		Context: ContextConfig{AutoGenerate: true, RefreshDays: 7, AnalysisDepth: "thorough"},
	}
}
