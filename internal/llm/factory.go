package llm

// New constructs the appropriate LLM backend for the given provider and wraps
// it in an AuditingLLM so that all calls are audit-logged.
func New(provider, apiKey, model string) LLM {
	var backend LLM
	switch provider {
	case "openai":
		backend = NewOpenAI(apiKey, model)
	default:
		backend = NewClaude(apiKey, model)
	}
	return NewAuditingLLM(backend)
}
