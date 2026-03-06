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
