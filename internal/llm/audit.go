package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditedLLM wraps any LLM and emits structured audit log entries for every call.
type AuditedLLM struct {
	inner LLM
}

// NewAudited wraps inner with audit logging. All callers should obtain LLM
// instances via the llm.New factory rather than constructing backends directly.
func NewAudited(inner LLM) LLM {
	return &AuditedLLM{inner: inner}
}

func (a *AuditedLLM) Name() string { return a.inner.Name() }

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	attrs := []any{
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"latency_ms", time.Since(start).Milliseconds(),
	}
	if err != nil {
		slog.Error("llm audit: Complete failed", append(attrs, "error", err)...)
	} else {
		slog.Info("llm audit: Complete", attrs...)
	}
	return resp, err
}

func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	slog.Info("llm audit: StreamComplete requested", "provider", a.inner.Name())
	return a.inner.StreamComplete(ctx, req)
}
