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
	AgentName   string    `json:"agent_name,omitempty"`
	System      string    `json:"system"`
	Messages    []Message `json:"messages"`
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

// allowedAgentNames is the allowlist of valid pipeline stage names for audit logging.
// Values not on this list are replaced with "unknown" to prevent inadvertent PII
// leakage if AgentName is ever populated from external input. Prompt content is
// intentionally excluded from log entries to prevent PHI/PII leakage.
var allowedAgentNames = map[string]bool{
	"architect": true,
	"security":  true,
	"review":    true,
}

// sanitizeAgentName returns name if it is on the allowlist, otherwise "unknown".
func sanitizeAgentName(name string) string {
	if allowedAgentNames[name] {
		return name
	}
	return "unknown"
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
