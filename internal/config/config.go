package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
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
	Kubernetes  KubernetesConfig
}

type KubernetesConfig struct {
	Namespace     string
	Image         string
	SecretName    string
	GitSecretName string
	Timeout       time.Duration
	CPU           string
	Memory        string
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
			Kubernetes: KubernetesConfig{
				Namespace:     envStr("NEURALFORGE_K8S_NAMESPACE", "neuralforge"),
				Image:         envStr("NEURALFORGE_K8S_IMAGE", "ghcr.io/neuralforge/claude-executor:latest"),
				SecretName:    envStr("NEURALFORGE_K8S_SECRET", "neuralforge-llm-keys"),
				GitSecretName: envStr("NEURALFORGE_K8S_GIT_SECRET", "neuralforge-git-token"),
				Timeout:       time.Duration(envInt("NEURALFORGE_K8S_TIMEOUT_MINUTES", 30)) * time.Minute,
				CPU:           envStr("NEURALFORGE_K8S_CPU", "2"),
				Memory:        envStr("NEURALFORGE_K8S_MEMORY", "4Gi"),
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

func (c Config) Validate() error {
	var errs []string

	// Server
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		errs = append(errs, fmt.Sprintf("invalid server port: %d (must be 1-65535)", c.Server.Port))
	}

	// Workers
	if c.Workers < 1 {
		errs = append(errs, fmt.Sprintf("invalid workers: %d (must be >= 1)", c.Workers))
	}

	// GitHub — required for core functionality
	if c.GitHub.AppID == 0 {
		errs = append(errs, "GITHUB_APP_ID is required")
	}
	if c.GitHub.PrivateKeyPath == "" {
		errs = append(errs, "GITHUB_PRIVATE_KEY_PATH is required")
	}
	if c.GitHub.WebhookSecret == "" {
		errs = append(errs, "GITHUB_WEBHOOK_SECRET is required")
	}

	// LLM — validate the selected provider has an API key
	switch c.LLM.DefaultProvider {
	case "claude":
		if c.LLM.Claude.APIKey == "" {
			errs = append(errs, "ANTHROPIC_API_KEY is required when provider is claude")
		}
	case "openai":
		if c.LLM.OpenAI.APIKey == "" {
			errs = append(errs, "OPENAI_API_KEY is required when provider is openai")
		}
	default:
		errs = append(errs, fmt.Sprintf("unknown LLM provider: %q (must be claude or openai)", c.LLM.DefaultProvider))
	}

	// Executor
	switch c.Executor.DefaultType {
	case "docker":
		if c.Executor.Docker.Timeout <= 0 {
			errs = append(errs, "docker timeout must be > 0")
		}
		if c.Executor.Docker.Image == "" {
			errs = append(errs, "docker image is required")
		}
	case "kubernetes":
		if c.Executor.Kubernetes.Timeout <= 0 {
			errs = append(errs, "kubernetes timeout must be > 0")
		}
		if c.Executor.Kubernetes.Image == "" {
			errs = append(errs, "kubernetes image is required")
		}
		if c.Executor.Kubernetes.Namespace == "" {
			errs = append(errs, "kubernetes namespace is required")
		}
	default:
		errs = append(errs, fmt.Sprintf("unknown executor type: %q (must be docker or kubernetes)", c.Executor.DefaultType))
	}

	// Store
	if c.Store.Driver == "" {
		errs = append(errs, "store driver is required")
	}
	if c.Store.DSN == "" {
		errs = append(errs, "store DSN is required")
	}

	// Context
	if c.Context.RefreshDays < 1 {
		errs = append(errs, fmt.Sprintf("invalid context refresh days: %d (must be >= 1)", c.Context.RefreshDays))
	}

	if len(errs) > 0 {
		return errors.New("config validation failed:\n  " + strings.Join(errs, "\n  "))
	}
	return nil
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
