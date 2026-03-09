package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditedLLM wraps any LLM and emits structured slog entries for every call.
type AuditedLLM struct {
	inner LLM
}

// NewAudited wraps inner with audit logging.
func NewAudited(inner LLM) *AuditedLLM {
	slog.Info("llm client created", "provider", inner.Name())
	return &AuditedLLM{inner: inner}
}

func (a *AuditedLLM) Name() string { return a.inner.Name() }

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	slog.Info("llm complete start",
		"provider", a.inner.Name(),
		"model", req.Model,
		"max_tokens", req.MaxTokens,
	)
	resp, err := a.inner.Complete(ctx, req)
	if err != nil {
		slog.Error("llm complete error",
			"provider", a.inner.Name(),
			"error", err,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return resp, err
	}
	slog.Info("llm complete ok",
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	return resp, nil
}

func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	slog.Info("llm stream start", "provider", a.inner.Name(), "model", req.Model)
	return a.inner.StreamComplete(ctx, req)
}

// New is the authoritative factory for creating audited LLM clients.
// provider is "openai" or "claude" (default). apiKey and model are forwarded
// to the backend constructor.
func New(provider, apiKey, model string) LLM {
	var backend LLM
	switch provider {
	case "openai":
		backend = NewOpenAI(apiKey, model)
	default:
		backend = NewClaude(apiKey, model)
	}
	return NewAudited(backend)
}
