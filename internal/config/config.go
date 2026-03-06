package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Workers  int
	GitHub   GitHubConfig
	LLM      LLMConfig
	Executor ExecutorConfig
	Store    StoreConfig
	Context  ContextConfig
}

type ServerConfig struct {
	Port int
	Host string
}

type GitHubConfig struct {
	AppID          int64
	PrivateKeyPath string
	WebhookSecret  string
}

type LLMConfig struct {
	DefaultProvider string
	Claude          ProviderConfig
	OpenAI          ProviderConfig
}

type ProviderConfig struct {
	APIKey string
	Model  string
}

type ExecutorConfig struct {
	DefaultType string
	Docker      DockerConfig
}

type DockerConfig struct {
	Image   string
	Timeout time.Duration
}

type StoreConfig struct {
	Driver string
	DSN    string
}

type ContextConfig struct {
	AutoGenerate  bool
	RefreshDays   int
	AnalysisDepth string
	CommitToRepo  bool
}

func LoadFromEnv() Config {
	return Config{
		Server: ServerConfig{
			Port: envInt("NEURALFORGE_PORT", 8080),
			Host: envStr("NEURALFORGE_HOST", "0.0.0.0"),
		},
		Workers: envInt("NEURALFORGE_WORKERS", 5),
		GitHub: GitHubConfig{
			AppID:          int64(envInt("GITHUB_APP_ID", 0)),
			PrivateKeyPath: envStr("GITHUB_PRIVATE_KEY_PATH", ""),
			WebhookSecret:  envStr("GITHUB_WEBHOOK_SECRET", ""),
		},
		LLM: LLMConfig{
			DefaultProvider: envStr("NEURALFORGE_LLM_PROVIDER", "claude"),
			Claude: ProviderConfig{
				APIKey: envStr("ANTHROPIC_API_KEY", ""),
				Model:  envStr("ANTHROPIC_MODEL", "claude-sonnet-4-5-20250514"),
			},
			OpenAI: ProviderConfig{
				APIKey: envStr("OPENAI_API_KEY", ""),
				Model:  envStr("OPENAI_MODEL", "gpt-4o"),
			},
		},
		Executor: ExecutorConfig{
			DefaultType: envStr("NEURALFORGE_EXECUTOR", "docker"),
			Docker: DockerConfig{
				Image:   envStr("NEURALFORGE_DOCKER_IMAGE", "ghcr.io/neuralforge/executor:latest"),
				Timeout: time.Duration(envInt("NEURALFORGE_TIMEOUT_MINUTES", 30)) * time.Minute,
			},
		},
		Store: StoreConfig{
			Driver: envStr("NEURALFORGE_STORE_DRIVER", "sqlite"),
			DSN:    envStr("NEURALFORGE_STORE_DSN", "neuralforge.db"),
		},
		Context: ContextConfig{
			AutoGenerate:  envBool("NEURALFORGE_AUTO_CONTEXT", true),
			RefreshDays:   envInt("NEURALFORGE_CONTEXT_REFRESH_DAYS", 7),
			AnalysisDepth: envStr("NEURALFORGE_ANALYSIS_DEPTH", "thorough"),
			CommitToRepo:  envBool("NEURALFORGE_CONTEXT_COMMIT", true),
		},
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
