package llm

import (
	"context"
	"fmt"
	"log/slog"
)

// New creates an LLM backend for the given provider and wraps it in an
// auditing layer that logs each completion call via slog.
// provider must be "openai" or "claude" (default).
//
// Audit log data-minimization: only request metadata (provider, model,
// token counts, cost) is logged — prompt content and completions are never
// written to the audit log, preventing inadvertent capture of PII/PHI that
// may appear in code diffs, issue bodies, or repository data.
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
	inner, err := a.inner.StreamComplete(ctx, req)
	if err != nil {
		slog.Warn("llm stream failed", "provider", a.inner.Name(), "model", req.Model, "error", err)
		return nil, err
	}

	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		for chunk := range inner {
			if chunk.Done {
				if chunk.Error != nil {
					slog.Warn("llm stream completed with error", "provider", a.inner.Name(), "model", req.Model, "error", chunk.Error)
				} else {
					slog.Info("llm stream completed", "provider", a.inner.Name(), "model", req.Model)
				}
			}
			out <- chunk
		}
	}()
	return out, nil
}
