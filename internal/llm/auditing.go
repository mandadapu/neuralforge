package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditingLLM wraps any LLM backend and emits structured audit log entries
// for every completion call, capturing provider, model, token usage, latency,
// and any error.
type AuditingLLM struct {
	inner LLM
}

// NewAuditingLLM wraps inner with audit logging. It is the sole approved way
// to obtain an LLM instance for production use.
func NewAuditingLLM(inner LLM) *AuditingLLM {
	return &AuditingLLM{inner: inner}
}

func (a *AuditingLLM) Name() string { return a.inner.Name() }

func (a *AuditingLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	latency := time.Since(start)
	if err != nil {
		slog.Error("llm complete",
			"provider", a.inner.Name(),
			"model", req.Model,
			"latency_ms", latency.Milliseconds(),
			"error", err,
		)
	} else {
		slog.Info("llm complete",
			"provider", a.inner.Name(),
			"model", resp.Model,
			"input_tokens", resp.InputTokens,
			"output_tokens", resp.OutputTokens,
			"cost_usd", resp.Cost,
			"latency_ms", latency.Milliseconds(),
		)
	}
	return resp, err
}

func (a *AuditingLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	slog.Info("llm stream_complete", "provider", a.inner.Name(), "model", req.Model)
	return a.inner.StreamComplete(ctx, req)
}
