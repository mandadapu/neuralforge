package llm

import (
	"fmt"
	"log/slog"
)

// New is the auditing factory for all LLM backends.
// It logs client instantiation before returning the backend so that
// every LLM client creation is captured in the audit trail.
func New(provider, apiKey, model string) (LLM, error) {
	slog.Info("llm client created", "provider", provider, "model", model)
	switch provider {
	case "openai":
		return newOpenAI(apiKey, model), nil
	case "claude":
		return newClaude(apiKey, model), nil
	default:
		return nil, fmt.Errorf("unknown llm provider: %s", provider)
	}
}
