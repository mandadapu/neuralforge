package llm

import "fmt"

// New creates an audited LLM backend for the given provider.
// It is the only sanctioned way to obtain an LLM instance; direct construction
// of OpenAIBackend or ClaudeBackend bypasses the audit layer.
func New(provider, apiKey, model string) (LLM, error) {
	var inner LLM
	switch provider {
	case "openai":
		inner = NewOpenAI(apiKey, model)
	case "claude", "":
		inner = NewClaude(apiKey, model)
	default:
		return nil, fmt.Errorf("unknown llm provider %q", provider)
	}
	return NewAudited(inner), nil
}
