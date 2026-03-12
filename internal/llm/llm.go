package llm

import "context"

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type CompletionRequest struct {
	// AgentName identifies the pipeline stage for local audit logging only.
	// json:"-" ensures this field is NEVER serialized or transmitted to any external LLM provider
	// (Anthropic, OpenAI, etc.). It is consumed solely by local slog instrumentation.
	// Values must be static pipeline-stage identifiers (e.g. "architect", "review", "security").
	// Do NOT populate this field with user-supplied input, PII, PHI, or financial data.
	AgentName string `json:"-"`
	System      string    `json:"system"`
	Messages    []Message `json:"messages"`
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

type CompletionResponse struct {
	Content      string `json:"content"`
	Model        string `json:"model"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	Cost         float64
}

type StreamChunk struct {
	Content string
	Done    bool
	Error   error
}

type LLM interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
	StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
	Name() string
}

// knownAgents is the allowlist of pipeline-stage identifiers permitted in audit logs.
// Only static, code-defined values appear here — never user-supplied input.
var knownAgents = map[string]struct{}{
	"architect": {},
	"review":    {},
	"security":  {},
}

// sanitizeAgentName returns the agent name if it is a known pipeline stage, or "[unknown]"
// otherwise. This prevents accidental logging of PII/PHI if a caller incorrectly populates
// AgentName with dynamic or user-supplied data.
func sanitizeAgentName(name string) string {
	if _, ok := knownAgents[name]; ok {
		return name
	}
	return "[unknown]"
}
