package llm

import (
	"context"
	"fmt"
	"log/slog"
)

// New creates an LLM backend for the given provider and wraps it in an
// auditing layer that logs each completion call via slog.
// provider must be "openai" or "claude" (default).
func New(provider, apiKey, model string) (LLM, error) {
	var backend LLM
	switch provider {
	case "openai":
		backend = NewOpenAI(apiKey, model)
	case "claude", "":
		backend = NewClaude(apiKey, model)
	default:
		return nil, fmt.Errorf("llm.New: unknown provider %q", provider)
	}

	slog.Info("llm client created", "provider", provider, "model", model)
	return &auditedLLM{inner: backend}, nil
}

// auditedLLM wraps any LLM and emits slog audit entries for each call.
type auditedLLM struct {
	inner LLM
}

func (a *auditedLLM) Name() string { return a.inner.Name() }

func (a *auditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	resp, err := a.inner.Complete(ctx, req)
	if err != nil {
		slog.Warn("llm completion failed", "provider", a.inner.Name(), "model", req.Model, "error", err)
		return resp, err
	}
	slog.Info("llm completion",
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
	)
	return resp, nil
}

func (a *auditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	return a.inner.StreamComplete(ctx, req)
}
