package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditingLLM wraps any LLM and emits a structured slog entry for every call.
type AuditingLLM struct {
	inner LLM
}

func (a *AuditingLLM) Name() string { return a.inner.Name() }

func (a *AuditingLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	slog.Info("llm.Complete",
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"latency_ms", time.Since(start).Milliseconds(),
		"error", err,
	)
	return resp, err
}

func (a *AuditingLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	slog.Info("llm.StreamComplete", "provider", a.inner.Name())
	return a.inner.StreamComplete(ctx, req)
}

// New creates an audited LLM for the given provider.
// provider must be "openai" or "claude" (default).
func New(provider, apiKey, model string) LLM {
	var inner LLM
	switch provider {
	case "openai":
		inner = NewOpenAI(apiKey, model)
	default:
		inner = NewClaude(apiKey, model)
	}
	return &AuditingLLM{inner: inner}
}
