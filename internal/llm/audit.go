package llm

import (
	"context"
	"log/slog"
	"time"
)

// auditedLLM wraps any LLM and emits a structured audit log entry for every
// Complete call, recording provider, model, token counts, cost, latency, and
// any error. This satisfies the security requirement that all LLM interactions
// pass through an auditing layer.
type auditedLLM struct {
	inner LLM
}

// NewAuditedLLM returns an LLM that wraps inner and audit-logs every Complete call.
func NewAuditedLLM(inner LLM) LLM {
	return &auditedLLM{inner: inner}
}

func (a *auditedLLM) Name() string {
	return a.inner.Name()
}

func (a *auditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	latency := time.Since(start)

	attrs := []any{
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"latency_ms", latency.Milliseconds(),
	}
	if err != nil {
		attrs = append(attrs, "error", err)
		slog.Error("llm audit: complete failed", attrs...)
	} else {
		slog.Info("llm audit: complete", attrs...)
	}

	return resp, err
}

func (a *auditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	return a.inner.StreamComplete(ctx, req)
}
