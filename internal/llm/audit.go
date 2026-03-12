package llm

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"
)

// AuditedLLM wraps any LLM and emits structured audit log entries on every call.
//
// # Content filtering
//
// Only operational metadata is logged (provider, model, token counts, cost, latency,
// audit_event). Message content and system prompts are intentionally excluded to
// prevent inadvertent logging of PHI/PII/PAN. Callers must not add content fields to
// this logger.
//
// # GDPR / data-retention basis
//
// The operational metrics logged by this wrapper (token counts, cost, latency, error
// codes) constitute pseudonymous technical telemetry with no direct personal
// identifiers. The legal basis for processing is Article 6(1)(f) GDPR (legitimate
// interests: capacity planning, fraud/abuse detection, cost management). Logs should
// be retained no longer than necessary per the organisation's retention policy.
// If LLMs process personal or health data, a DPIA must be conducted per Article 35
// GDPR before deployment to assess re-identification risks from operational metadata.
//
// # HIPAA note
//
// Authentication and authorisation failures are emitted with the audit code
// "llm_auth_error" to support HIPAA §164.312(b) audit control requirements.
type AuditedLLM struct {
	inner LLM
}

// NewAudited returns an LLM that wraps inner with audit logging.
func NewAudited(inner LLM) LLM {
	return &AuditedLLM{inner: inner}
}

func (a *AuditedLLM) Name() string { return a.inner.Name() }

// auditEvent classifies a call outcome for HIPAA §164.312(b) audit trail purposes.
func auditEvent(err error) string {
	if err == nil {
		return "llm_complete_ok"
	}
	// Classify authentication/authorisation failures separately.
	var httpErr interface{ StatusCode() int }
	if errors.As(err, &httpErr) {
		code := httpErr.StatusCode()
		if code == http.StatusUnauthorized || code == http.StatusForbidden {
			return "llm_auth_error"
		}
	}
	return "llm_complete_error"
}

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	// Note: req.Messages and resp.Content are intentionally omitted to prevent
	// logging PHI/PII/PAN. Only operational metadata is recorded.
	slog.Info("llm.Complete",
		"audit_event", auditEvent(err),
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"latency_ms", time.Since(start).Milliseconds(),
		"error", err,
	)
	return resp, err
}

// StreamComplete wraps the inner stream and emits a completion audit entry when the
// stream closes, including latency. Content chunks are intentionally not logged to
// prevent PHI/PII/PAN exposure.
func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	inner, err := a.inner.StreamComplete(ctx, req)
	if err != nil {
		slog.Info("llm.StreamComplete",
			"audit_event", auditEvent(err),
			"provider", a.inner.Name(),
			"error", err,
		)
		return nil, err
	}

	start := time.Now()
	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		var lastErr error
		for chunk := range inner {
			out <- chunk
			if chunk.Error != nil {
				lastErr = chunk.Error
			}
		}
		// Emit audit entry at stream closure. Content is intentionally omitted.
		slog.Info("llm.StreamComplete",
			"audit_event", auditEvent(lastErr),
			"provider", a.inner.Name(),
			"latency_ms", time.Since(start).Milliseconds(),
			"error", lastErr,
		)
	}()
	return out, nil
}
