package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditingLLM wraps an LLM backend and emits a structured audit log entry
// for every completion call, recording provider, model, token usage, cost,
// latency, and any error.
type AuditingLLM struct {
	inner LLM
}

// NewAudited wraps inner with audit logging. Every Complete call will emit
// a slog entry at Info level with key "llm_audit".
func NewAudited(inner LLM) LLM {
	return &AuditingLLM{inner: inner}
}

func (a *AuditingLLM) Name() string { return a.inner.Name() }

func (a *AuditingLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	slog.InfoContext(ctx, "llm_audit",
		"provider", a.inner.Name(),
		"model", req.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"latency_ms", time.Since(start).Milliseconds(),
		"error", err,
	)
	return resp, err
}

func (a *AuditingLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	return a.inner.StreamComplete(ctx, req)
}
