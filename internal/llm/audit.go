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
	// DATA CLASSIFICATION: fields below are Class-3 operational metrics only.
	// Prompt content, system instructions, and response text are intentionally
	// excluded to prevent PII/PHI exposure in logs.
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
	start := time.Now()
	ch, err := a.inner.StreamComplete(ctx, req)
	// Sanitize the initiation error before logging (same rationale as Complete).
	if err != nil {
		slog.Info("llm.StreamComplete",
			"provider", a.inner.Name(),
			"error", err.Error(),
		)
		return nil, err
	}

	// Guard against an unexpected nil channel with no error from the inner
	// implementation. A nil channel would block the goroutine forever.
	if ch == nil {
		slog.Info("llm.StreamComplete",
			"provider", a.inner.Name(),
			"chunks", 0,
			"latency_ms", time.Since(start).Milliseconds(),
			"error", "inner returned nil channel",
		)
		empty := make(chan StreamChunk)
		close(empty)
		return empty, nil
	}

	// Wrap the upstream channel so we can log completion metadata once the
	// stream finishes. Only operational metadata (chunk count, latency, error
	// message) is recorded — no prompt or response content is ever logged.
	// DATA CLASSIFICATION: fields logged below are Class-3 operational metrics,
	// not PII/PHI. If the upstream implementation ever surfaces user content in
	// StreamChunk.Error, sanitize it here before logging.
	wrapped := make(chan StreamChunk)
	go func() {
		defer close(wrapped)
		var chunks int
		var streamErr string
		for chunk := range ch {
			chunks++
			if chunk.Error != nil {
				// Sanitize: log only the error message string.
				streamErr = chunk.Error.Error()
			}
			wrapped <- chunk
		}
		slog.Info("llm.StreamComplete",
			"provider", a.inner.Name(),
			"chunks", chunks,
			"latency_ms", time.Since(start).Milliseconds(),
			"error", streamErr,
		)
	}()
	return wrapped, nil
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
