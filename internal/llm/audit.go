package llm

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// AuditingLLM wraps any LLM and emits a structured slog entry for every call.
// NOTE: The slog handler destination must be configured to use encrypted,
// access-controlled storage to meet GDPR/HIPAA requirements where applicable.
// Request prompt content is never logged — only response metadata (model,
// token counts, cost) is recorded.
type AuditingLLM struct {
	inner LLM
}

func (a *AuditingLLM) Name() string { return a.inner.Name() }

func (a *AuditingLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	// Sanitize the error: log only the message string to avoid leaking
	// internal state, stack traces, or sensitive data embedded in error values.
	var errMsg any
	if err != nil {
		errMsg = err.Error()
	}
	slog.Info("llm.Complete",
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"latency_ms", time.Since(start).Milliseconds(),
		"error", errMsg,
	)
	return resp, err
}

func (a *AuditingLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	ch, err := a.inner.StreamComplete(ctx, req)
	// Sanitize the error before logging (same rationale as Complete).
	var errMsg any
	if err != nil {
		errMsg = err.Error()
	}
	slog.Info("llm.StreamComplete",
		"provider", a.inner.Name(),
		"error", errMsg,
	)
	return ch, err
}

// New creates an audited LLM for the given provider.
// provider must be "openai" or "claude"; any other value returns an error.
func New(provider, apiKey, model string) (LLM, error) {
	var inner LLM
	switch provider {
	case "openai":
		inner = NewOpenAI(apiKey, model)
	case "claude":
		inner = NewClaude(apiKey, model)
	default:
		return nil, fmt.Errorf("llm.New: unknown provider %q (must be \"openai\" or \"claude\")", provider)
	}
	return &AuditingLLM{inner: inner}, nil
}
