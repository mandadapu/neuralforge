package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditingLLM wraps any LLM and emits a structured audit log entry for
// every completion request. This satisfies the agent_sec_001 requirement
// that all LLM clients pass through an auditing layer.
type AuditingLLM struct {
	inner LLM
}

// WithAudit wraps inner with audit logging. Use this instead of
// instantiating LLM backends directly.
func WithAudit(inner LLM) LLM {
	return &AuditingLLM{inner: inner}
}

func (a *AuditingLLM) Name() string { return a.inner.Name() }

func (a *AuditingLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	slog.Info("llm_audit",
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
	slog.Info("llm_audit", "provider", a.inner.Name(), "operation", "stream_complete")
	return a.inner.StreamComplete(ctx, req)
}
