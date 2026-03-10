package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditedLLM wraps an LLM and emits structured audit log entries for every
// completion call, satisfying the security requirement that all LLM client
// usage passes through an auditing layer.
type AuditedLLM struct {
	inner LLM
}

// NewAudited returns an LLM that wraps inner with audit logging.
// All callers must use this factory instead of instantiating backends directly.
func NewAudited(inner LLM) LLM {
	return &AuditedLLM{inner: inner}
}

func (a *AuditedLLM) Name() string { return a.inner.Name() }

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	slog.Info("llm.Complete started",
		"provider", a.inner.Name(),
		"model", req.Model,
		"message_count", len(req.Messages),
		"has_system", req.System != "",
	)

	resp, err := a.inner.Complete(ctx, req)

	attrs := []any{
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"duration_ms", time.Since(start).Milliseconds(),
	}
	if err != nil {
		slog.Error("llm.Complete failed", append(attrs, "error", err)...)
	} else {
		slog.Info("llm.Complete finished", attrs...)
	}
	return resp, err
}

func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	slog.Info("llm.StreamComplete started",
		"provider", a.inner.Name(),
		"model", req.Model,
	)
	ch, err := a.inner.StreamComplete(ctx, req)
	if err != nil {
		slog.Error("llm.StreamComplete failed", "provider", a.inner.Name(), "error", err)
	}
	return ch, err
}
