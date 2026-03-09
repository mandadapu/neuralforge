package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditedLLM wraps any LLM backend and emits structured audit log entries
// for every completion call, capturing provider, model, token usage, cost,
// latency, and error outcome.
type AuditedLLM struct {
	backend LLM
	logger  *slog.Logger
}

// NewAudited returns an LLM that logs each call through logger before
// delegating to backend. Callers should always instantiate LLM clients
// through this function rather than constructing backends directly.
func NewAudited(backend LLM, logger *slog.Logger) LLM {
	return &AuditedLLM{backend: backend, logger: logger}
}

func (a *AuditedLLM) Name() string { return a.backend.Name() }

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.backend.Complete(ctx, req)
	attrs := []any{
		"provider",      a.backend.Name(),
		"model",         req.Model,
		"input_tokens",  resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd",      resp.Cost,
		"latency_ms",    time.Since(start).Milliseconds(),
		"error",         err,
	}
	if err != nil {
		a.logger.ErrorContext(ctx, "llm_complete", attrs...)
	} else {
		a.logger.InfoContext(ctx, "llm_complete", attrs...)
	}
	return resp, err
}

func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	a.logger.InfoContext(ctx, "llm_stream_complete", "provider", a.backend.Name(), "model", req.Model)
	return a.backend.StreamComplete(ctx, req)
}
