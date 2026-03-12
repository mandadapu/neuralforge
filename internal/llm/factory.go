package llm

import "github.com/mandadapu/neuralforge/internal/config"

// NewFromConfig constructs the appropriate LLM backend from cfg and wraps it
// with audit logging, satisfying the agent_sec_001 auditing factory requirement.
func NewFromConfig(cfg config.LLMConfig) LLM {
	var backend LLM
	switch cfg.DefaultProvider {
	case "openai":
		backend = NewOpenAI(cfg.OpenAI.APIKey, cfg.OpenAI.Model)
	default:
		backend = NewClaude(cfg.Claude.APIKey, cfg.Claude.Model)
	}
	return WithAudit(backend)
}
