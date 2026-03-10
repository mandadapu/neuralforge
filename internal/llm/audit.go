package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditedLLM wraps any LLM and emits a structured audit log entry on every call.
type AuditedLLM struct {
	inner LLM
}

// NewAudited returns an LLM that wraps inner with audit logging.
func NewAudited(inner LLM) LLM {
	return &AuditedLLM{inner: inner}
}

func (a *AuditedLLM) Name() string { return a.inner.Name() }

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
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

func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	slog.Info("llm.StreamComplete", "provider", a.inner.Name())
	return a.inner.StreamComplete(ctx, req)
}
