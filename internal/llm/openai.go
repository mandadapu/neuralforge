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
		// Security control (llm_004): max_tokens MUST always be set to prevent
		// resource exhaustion. MaxTokens=0 is not a supported "no limit" opt-out;
		// it triggers this mandatory bounded-default enforcement.
		//
		// Compliance rationale for default value of 4096:
		//   - GDPR Art. 5(1)(c) data minimization: a hard upper bound (any finite
		//     limit) satisfies the requirement better than unbounded output; 4096 is
		//     the established pipeline-wide ceiling set explicitly in every stage.
		//   - HIPAA minimum-necessary: callers that process PHI MUST supply an
		//     explicit req.MaxTokens appropriate for their data access policy —
		//     do not rely on this default for PHI use cases.
		//   - SOX cost-control: see audit log below for traceability of when this
		//     default fires vs. an explicit caller-supplied limit.
		//   - 4096 matches ClaudeBackend's default, keeping all backends consistent.
		maxTokens = 4096
		log.Printf("[llm_004][security-control] openai: max_tokens enforced as default=%d for model=%s; caller supplied MaxTokens=0 (no explicit limit). PHI callers must set an explicit lower limit per their data access policy.", maxTokens, model)
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
