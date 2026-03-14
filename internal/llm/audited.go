package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditedBackend wraps an LLM and logs every call with provider, model, tokens, cost, and duration.
type AuditedBackend struct {
	inner LLM
}

// NewAudited wraps any LLM backend with audit logging.
func NewAudited(inner LLM) *AuditedBackend {
	return &AuditedBackend{inner: inner}
}

// NewAuditedOpenAI creates an OpenAI backend wrapped with audit logging.
func NewAuditedOpenAI(apiKey, model string) *AuditedBackend {
	return NewAudited(NewOpenAI(apiKey, model))
}

// NewAuditedClaude creates a Claude backend wrapped with audit logging.
func NewAuditedClaude(apiKey, model string) *AuditedBackend {
	return NewAudited(NewClaude(apiKey, model))
}

func (a *AuditedBackend) Name() string {
	return a.inner.Name()
}

func (a *AuditedBackend) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	slog.Info("llm call started", "provider", a.inner.Name(), "model", req.Model)

	resp, err := a.inner.Complete(ctx, req)
	duration := time.Since(start)

	if err != nil {
		slog.Error("llm call failed", "provider", a.inner.Name(), "duration", duration, "error", err)
		return resp, err
	}

	slog.Info("llm call completed",
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost", resp.Cost,
		"duration", duration,
	)
	return resp, nil
}

func (a *AuditedBackend) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	slog.Info("llm stream started", "provider", a.inner.Name(), "model", req.Model)
	return a.inner.StreamComplete(ctx, req)
}
