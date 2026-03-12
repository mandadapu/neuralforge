package llm

import (
	"fmt"
	"log/slog"
	"strings"
	"time"
)

// New is the auditing factory for all LLM backends.
// It logs client instantiation after the backend is successfully created so
// that the audit entry only appears when creation actually succeeded.
// The apiKey parameter is intentionally excluded from the audit log to prevent
// credential exposure; only provider and model are recorded.
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
	// Explicit UTC timestamp is included to satisfy audit trail requirements
	// (GDPR Art. 30, HIPAA §164.312(b), SOX 404) independent of log handler config.
	// model is sanitized before logging to prevent log injection.
	// apiKey is intentionally omitted to prevent credential exposure.
	slog.Info("llm client created",
		"provider", provider,
		"model", sanitizeModel(model),
		"audit", true,
		"timestamp", time.Now().UTC().Format(time.RFC3339),
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
