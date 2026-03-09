package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditingLLM wraps any LLM backend and emits audit log entries for every call.
// Note: StreamComplete logs only the invocation; per-chunk results cannot be
// audited because the channel is returned asynchronously.
type AuditingLLM struct {
	inner LLM
}

func NewAuditingLLM(inner LLM) *AuditingLLM {
	return &AuditingLLM{inner: inner}
}

func (a *AuditingLLM) Name() string { return a.inner.Name() }

func (a *AuditingLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	slog.Info("llm.Complete called",
		"provider", a.inner.Name(),
		"model", req.Model,
		"messages", len(req.Messages),
	)
	resp, err := a.inner.Complete(ctx, req)
	if err != nil {
		slog.Error("llm.Complete failed",
			"provider", a.inner.Name(),
			"model", req.Model,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", err,
		)
		return resp, err
	}
	slog.Info("llm.Complete succeeded",
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return resp, nil
}

func (a *AuditingLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	slog.Info("llm.StreamComplete called", "provider", a.inner.Name(), "model", req.Model)
	return a.inner.StreamComplete(ctx, req)
}
