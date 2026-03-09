package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditedLLM wraps an LLM backend and emits structured audit log entries
// for every completion request: provider, model, token counts, cost, and latency.
type AuditedLLM struct {
	inner LLM
}

// NewAudited returns an LLM that wraps inner with audit logging.
// All call sites must go through this function instead of instantiating
// backends directly, satisfying the auditing factory requirement.
func NewAudited(inner LLM) LLM {
	return &AuditedLLM{inner: inner}
}

func (a *AuditedLLM) Name() string {
	return a.inner.Name()
}

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	elapsed := time.Since(start)

	attrs := []any{
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"latency_ms", elapsed.Milliseconds(),
	}
	if err != nil {
		attrs = append(attrs, "error", err)
		slog.WarnContext(ctx, "llm.Complete failed", attrs...)
	} else {
		slog.InfoContext(ctx, "llm.Complete", attrs...)
	}
	return resp, err
}

func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	slog.InfoContext(ctx, "llm.StreamComplete", "provider", a.inner.Name())
	return a.inner.StreamComplete(ctx, req)
}
