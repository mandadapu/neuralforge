package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditedLLM wraps an inner LLM and logs audit information for every call.
type AuditedLLM struct {
	inner  LLM
	logger *slog.Logger
}

// NewAuditedLLM returns an LLM that delegates to inner and logs audit events.
// If logger is nil, slog.Default() is used.
func NewAuditedLLM(inner LLM, logger *slog.Logger) LLM {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuditedLLM{inner: inner, logger: logger}
}

func (a *AuditedLLM) Name() string {
	return "audited/" + a.inner.Name()
}

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	a.logger.Info("llm.call.start", "provider", a.inner.Name(), "model", req.Model, "messages", len(req.Messages))
	resp, err := a.inner.Complete(ctx, req)
	elapsed := time.Since(start)
	if err != nil {
		a.logger.Error("llm.call.error", "provider", a.inner.Name(), "model", req.Model, "duration_ms", elapsed.Milliseconds(), "error", err)
		return resp, err
	}
	a.logger.Info("llm.call.done", "provider", a.inner.Name(), "model", resp.Model,
		"input_tokens", resp.InputTokens, "output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost, "duration_ms", elapsed.Milliseconds())
	return resp, nil
}

func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	a.logger.Info("llm.stream.start", "provider", a.inner.Name(), "model", req.Model)
	return a.inner.StreamComplete(ctx, req)
}
