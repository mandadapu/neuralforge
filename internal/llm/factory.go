package llm

import (
	"fmt"
	"log/slog"
	"strings"
)

// New is the auditing factory for all LLM backends.
// It logs client instantiation after the backend is successfully created so
// that the audit entry only appears when creation actually succeeded.
// The apiKey is never passed to the log call to prevent credential exposure.
func New(provider, apiKey, model string) (LLM, error) {
	var backend LLM
	switch provider {
	case "openai":
		backend = newOpenAI(apiKey, model)
	case "claude":
		backend = newClaude(apiKey, model)
	default:
		return nil, fmt.Errorf("unknown llm provider: %s", provider)
	}

	// Audit log emitted after successful instantiation.
	// model is sanitized before logging to prevent log injection.
	slog.Info("llm client created",
		"provider", provider,
		"model", sanitizeModel(model),
		"audit", true,
	)
	return backend, nil
}

// sanitizeModel strips characters not expected in a model identifier to
// prevent log injection or unintended exposure of sensitive configuration.
func sanitizeModel(model string) string {
	var b strings.Builder
	for _, r := range model {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '.' || r == '_' {
			b.WriteRune(r)
		}
	}
	return b.String()
}
