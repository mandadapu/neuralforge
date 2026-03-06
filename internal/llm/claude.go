package llm

import (
	"context"
	"fmt"

	anthropic "github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
)

type ClaudeBackend struct {
	client anthropic.Client
	model  string
}

func NewClaude(apiKey, model string) *ClaudeBackend {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	return &ClaudeBackend{
		client: client,
		model:  model,
	}
}

func (c *ClaudeBackend) Name() string {
	return "claude"
}

func (c *ClaudeBackend) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}

	maxTokens := int64(req.MaxTokens)
	if maxTokens == 0 {
		maxTokens = 4096
	}

	// Build message params from request messages
	var msgs []anthropic.MessageParam
	for _, m := range req.Messages {
		switch m.Role {
		case RoleUser:
			msgs = append(msgs, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content)))
		case RoleAssistant:
			msgs = append(msgs, anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content)))
		case RoleSystem:
			// System messages are handled via the System field on the params
			continue
		}
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(model),
		MaxTokens: maxTokens,
		Messages:  msgs,
	}

	// Set system prompt if provided
	if req.System != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: req.System},
		}
	}

	// Set temperature if non-zero
	if req.Temperature > 0 {
		params.Temperature = param.NewOpt(req.Temperature)
	}

	resp, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("claude completion failed: %w", err)
	}

	// Extract text content from response
	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	inputTokens := int(resp.Usage.InputTokens)
	outputTokens := int(resp.Usage.OutputTokens)

	return CompletionResponse{
		Content:      content,
		Model:        string(resp.Model),
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         CalculateCost(model, inputTokens, outputTokens),
	}, nil
}

func (c *ClaudeBackend) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	return nil, fmt.Errorf("streaming not implemented for claude backend")
}
