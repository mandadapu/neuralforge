// Package llm provides LLM client implementations and an auditing wrapper.
//
// # Audit Logging — Data Minimization Policy
//
// AuditedLLM logs only operational metadata to satisfy HIPAA §164.312(b),
// GDPR Article 32, and PCI-DSS Requirement 3.4. The following fields are
// deliberately excluded from all log entries to prevent PHI/PAN/personal-data
// leakage:
//
//   - CompletionRequest.System     (may contain user instructions with PII)
//   - CompletionRequest.Messages   (may contain PHI, PAN, or personal data)
//   - CompletionResponse.Content   (may contain derived/reflected sensitive data)
//
// Only the following non-content operational fields are logged:
//
//   - provider name, model identifier, max_tokens (request sizing)
//   - input_tokens, output_tokens, cost_usd, duration_ms (billing/performance)
//
// These fields contain no user-supplied data and carry negligible re-identification
// risk. If the deployment context requires stricter controls (e.g., zero-log mode),
// replace NewAudited with a no-op wrapper before calling llm.New.
package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditedLLM wraps any LLM and emits structured slog entries for every call.
// See package-level documentation for the data minimization policy.
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
	start := time.Now()
	provider := a.inner.Name()
	slog.Info("llm stream start", "provider", provider, "model", req.Model)

	inner, err := a.inner.StreamComplete(ctx, req)
	if err != nil {
		slog.Error("llm stream error",
			"provider", provider,
			"model", req.Model,
			"error", err,
			"duration_ms", time.Since(start).Milliseconds(),
		)
		return nil, err
	}

	// Wrap the channel so we can emit a completion audit event when streaming ends.
	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		for chunk := range inner {
			out <- chunk
			if chunk.Done {
				slog.Info("llm stream ok",
					"provider", provider,
					"model", req.Model,
					"duration_ms", time.Since(start).Milliseconds(),
				)
			}
			if chunk.Error != nil {
				slog.Error("llm stream chunk error",
					"provider", provider,
					"model", req.Model,
					"error", chunk.Error,
					"duration_ms", time.Since(start).Milliseconds(),
				)
			}
		}
	}()
	return out, nil
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
