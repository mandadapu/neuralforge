package llm

import (
	"context"
	"fmt"
	"log/slog"
)

// AuditingLLM wraps any LLM and emits a structured audit log entry
// for every completion request (provider, model, token usage, cost).
type AuditingLLM struct {
	inner LLM
}

func (a *AuditingLLM) Name() string { return a.inner.Name() }

func (a *AuditingLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	resp, err := a.inner.Complete(ctx, req)
	if err != nil {
		slog.Error("llm completion failed", "provider", a.inner.Name(), "error", err)
		return resp, err
	}
	slog.Info("llm completion",
		"provider",      a.inner.Name(),
		"model",         resp.Model,
		"input_tokens",  resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd",      resp.Cost,
	)
	return resp, nil
}

func (a *AuditingLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	slog.Info("llm stream_complete", "provider", a.inner.Name())
	return a.inner.StreamComplete(ctx, req)
}

// NewFactory creates the appropriate LLM backend for the given provider and wraps
// it in an AuditingLLM so every call is audit-logged through a single path.
func NewFactory(provider, apiKey, model string) (LLM, error) {
	var inner LLM
	switch provider {
	case "openai":
		inner = NewOpenAI(apiKey, model)
	case "claude", "":
		inner = NewClaude(apiKey, model)
	default:
		return nil, fmt.Errorf("unknown LLM provider: %q", provider)
	}
	return &AuditingLLM{inner: inner}, nil
}
