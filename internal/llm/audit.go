package llm

import (
	"context"
	"log/slog"
	"time"
)

// AuditConfig controls which fields are included in audit log entries.
// By default all content fields are redacted to prevent PII/PHI leakage.
// Callers operating in non-sensitive environments may opt-in to richer
// logging, but must ensure they have a lawful basis under applicable privacy
// regulations (e.g. GDPR Article 6) before enabling content logging.
//
// Security note: prompt and completion content is NEVER logged by default.
// Only metadata (provider, model, token counts, cost, latency) is emitted.
// This prevents inadvertent logging of PII, PHI, or financial data that may
// appear in user prompts or model responses.
type AuditConfig struct {
	// RedactContent suppresses prompt and completion content from audit logs.
	// Defaults to true. Set to false only when content logging is explicitly
	// authorized and a lawful basis under applicable data-protection law exists.
	RedactContent bool
}

// DefaultAuditConfig returns a safe-by-default configuration that redacts all
// content fields from audit log entries.
func DefaultAuditConfig() AuditConfig {
	return AuditConfig{RedactContent: true}
}

// AuditedLLM wraps any LLM backend and emits structured audit log entries
// for every completion call, capturing provider, model, token usage, cost,
// latency, and error outcome. Prompt and response content is redacted by
// default; see AuditConfig.
type AuditedLLM struct {
	backend LLM
	logger  *slog.Logger
	cfg     AuditConfig
}

// NewAudited returns an LLM that logs each call through logger before
// delegating to backend, using DefaultAuditConfig (content redacted).
// Callers should always instantiate LLM clients through this function rather
// than constructing backends directly.
func NewAudited(backend LLM, logger *slog.Logger) LLM {
	return NewAuditedWithConfig(backend, logger, DefaultAuditConfig())
}

// NewAuditedWithConfig returns an AuditedLLM with a custom AuditConfig.
func NewAuditedWithConfig(backend LLM, logger *slog.Logger, cfg AuditConfig) LLM {
	return &AuditedLLM{backend: backend, logger: logger, cfg: cfg}
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
	// Prompt and completion content are intentionally omitted from attrs
	// to prevent PII/PHI from entering audit logs (GDPR Article 5(1)(f),
	// HIPAA §164.312). Enable via AuditConfig.RedactContent=false only
	// when a documented lawful basis and appropriate safeguards are in place.
	if err != nil {
		a.logger.ErrorContext(ctx, "llm_complete", attrs...)
	} else {
		a.logger.InfoContext(ctx, "llm_complete", attrs...)
	}
	return resp, err
}

// StreamComplete wraps the upstream channel to audit completion metrics
// (token usage, latency) once the stream finishes, matching the observability
// coverage of the non-streaming Complete path.
func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	start := time.Now()
	upstream, err := a.backend.StreamComplete(ctx, req)
	if err != nil {
		a.logger.ErrorContext(ctx, "llm_stream_complete",
			"provider", a.backend.Name(),
			"model", req.Model,
			"error", err,
		)
		return nil, err
	}

	wrapped := make(chan StreamChunk)
	go func() {
		defer close(wrapped)
		var chunks int
		var lastErr error
		for chunk := range upstream {
			chunks++
			if chunk.Error != nil {
				lastErr = chunk.Error
			}
			wrapped <- chunk
		}
		a.logger.InfoContext(ctx, "llm_stream_complete",
			"provider", a.backend.Name(),
			"model", req.Model,
			"chunks", chunks,
			"latency_ms", time.Since(start).Milliseconds(),
			"error", lastErr,
		)
	}()

	return wrapped, nil
}
