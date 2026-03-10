package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditedLLM wraps any LLM backend and emits structured audit log entries
// on every completion call: provider, model, token counts, cost, latency,
// and any error.
type AuditedLLM struct {
	inner LLM
}

// NewAuditedLLM returns an LLM that delegates to inner but logs each call.
// All LLM backends must be constructed through this factory so that audit
// controls cannot be bypassed.
func NewAuditedLLM(inner LLM) LLM {
	return &AuditedLLM{inner: inner}
}

func (a *AuditedLLM) Name() string { return a.inner.Name() }

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	slog.Info("llm_call",
		"provider",      a.inner.Name(),
		"model",         resp.Model,
		"input_tokens",  resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd",      resp.Cost,
		"latency_ms",    time.Since(start).Milliseconds(),
		"error",         err,
	)
	return resp, err
}

func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	slog.Info("llm_stream_call", "provider", a.inner.Name())
	return a.inner.StreamComplete(ctx, req)
}
