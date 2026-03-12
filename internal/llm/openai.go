package llm

import (
	"context"
	"fmt"
	"log"

	openai "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

type OpenAIBackend struct {
	client openai.Client
	model  string
}

func NewOpenAI(apiKey, model string) *OpenAIBackend {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	return &OpenAIBackend{
		client: client,
		model:  model,
	}
}

func (o *OpenAIBackend) Name() string {
	return "openai"
}

func (o *OpenAIBackend) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = o.model
	}

	// Build messages, prepending system message if provided
	var msgs []openai.ChatCompletionMessageParamUnion
	if req.System != "" {
		msgs = append(msgs, openai.SystemMessage(req.System))
	}
	for _, m := range req.Messages {
		switch m.Role {
		case RoleUser:
			msgs = append(msgs, openai.UserMessage(m.Content))
		case RoleAssistant:
			msgs = append(msgs, openai.AssistantMessage(m.Content))
		case RoleSystem:
			msgs = append(msgs, openai.SystemMessage(m.Content))
		}
	}

	maxTokens := int64(req.MaxTokens)
	if maxTokens == 0 {
		// Default to 4096 tokens when the caller does not specify a limit.
		// Rationale (llm_004 / data-minimization compliance):
		//   - An unbounded request (MaxTokens=0) risks resource exhaustion and
		//     may extract more data than necessary, violating data-minimization
		//     principles (GDPR Art. 5(1)(c)) and HIPAA minimum-necessary rules.
		//   - 4096 matches the ClaudeBackend default and is the established
		//     pipeline-wide ceiling already set explicitly in every pipeline stage.
		//   - Callers that require a higher or lower limit must set req.MaxTokens
		//     explicitly; 0 is not a supported opt-out for "no limit".
		// Logged here for SOX cost-control audit trail.
		maxTokens = 4096
		log.Printf("openai: max_tokens defaulted to %d for model=%s (caller did not set MaxTokens; explicit limit required to override)", maxTokens, model)
	}

	params := openai.ChatCompletionNewParams{
		Model:               model,
		Messages:            msgs,
		MaxCompletionTokens: param.NewOpt(maxTokens),
	}

	if req.Temperature > 0 {
		params.Temperature = param.NewOpt(req.Temperature)
	}

	resp, err := withRetry(ctx, DefaultRetryConfig, func() (*openai.ChatCompletion, error) {
		return o.client.Chat.Completions.New(ctx, params)
	})
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("openai completion failed: %w", err)
	}

	var content string
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	inputTokens := int(resp.Usage.PromptTokens)
	outputTokens := int(resp.Usage.CompletionTokens)

	return CompletionResponse{
		Content:      content,
		Model:        resp.Model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         CalculateCost(model, inputTokens, outputTokens),
	}, nil
}

func (o *OpenAIBackend) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	return nil, fmt.Errorf("streaming not implemented for openai backend")
}
