package llm

import "fmt"

// allowedProviders is the authoritative allowlist of LLM providers permitted
// for use in this system. Requests for any other provider are rejected to
// prevent unauthorized backends from being instantiated.
var allowedProviders = map[string]bool{
	"openai":    true,
	"claude":    true,
	"anthropic": true,
}

// New constructs the appropriate LLM backend for the given provider and wraps
// it in an AuditingLLM so that all calls are audit-logged.
// An error is returned if the provider is not in the allowed list (RBAC check).
func New(provider, apiKey, model string) (LLM, error) {
	if !allowedProviders[provider] {
		return nil, fmt.Errorf("llm.New: provider %q is not permitted", provider)
	}
	var backend LLM
	switch provider {
	case "openai":
		backend = NewOpenAI(apiKey, model)
	default:
		backend = NewClaude(apiKey, model)
	}
	return NewAuditingLLM(backend), nil
}
