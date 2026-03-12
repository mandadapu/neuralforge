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
// Data minimization (GDPR Article 5(1)(c), HIPAA Safe Harbor): only request
// metadata is logged — provider, model, token counts, and cost. Prompt content,
// system prompts, and completion text are NEVER written to the audit log,
// preventing inadvertent capture of PII/PHI that may appear in code diffs,
// issue bodies, or repository data.
//
// Legal basis for logging (GDPR Article 6(1)(f)): audit entries constitute a
// legitimate interest for security monitoring, abuse prevention, and cost
// attribution. Logs should be retained according to your organization's data
// retention policy (recommended: ≤90 days for operational logs) and must not
// be shared with third parties without a separate legal basis.
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
		slog.WarnContext(ctx, "llm completion failed", "provider", a.inner.Name(), "model", req.Model, "error", err)
		return resp, err
	}
	slog.InfoContext(ctx, "llm completion",
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
		slog.WarnContext(ctx, "llm stream failed", "provider", a.inner.Name(), "model", req.Model, "error", err)
		return nil, err
	}

	slog.InfoContext(ctx, "llm stream started", "provider", a.inner.Name(), "model", req.Model)

	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		for chunk := range inner {
			if chunk.Done {
				if chunk.Error != nil {
					slog.WarnContext(ctx, "llm stream completed with error", "provider", a.inner.Name(), "model", req.Model, "error", chunk.Error)
				} else {
					slog.InfoContext(ctx, "llm stream completed", "provider", a.inner.Name(), "model", req.Model)
				}
			}
			out <- chunk
		}
	}()
	return out, nil
}
