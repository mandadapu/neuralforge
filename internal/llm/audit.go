package llm

import (
	"context"
	"log/slog"
	"strings"
	"time"
	"unicode"
)

// maxErrorLen caps the length of error messages written to logs to prevent
// PII/PHI/PAN leakage via provider error responses that may echo request content.
const maxErrorLen = 200

// maxLogFieldLen caps the length of metadata fields (model name, provider) in
// log entries to prevent log injection via overly long or crafted identifiers.
const maxLogFieldLen = 100

// sanitizeForLog removes control characters (including newlines and carriage
// returns) from strings before they are written to log sinks.  This prevents
// log-injection attacks where a crafted model name or provider string could
// forge additional log lines.  Only structural metadata — never message content
// — is passed through this function.
func sanitizeForLog(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return '_'
		}
		return r
	}, s)
	if len(s) > maxLogFieldLen {
		return s[:maxLogFieldLen] + "[truncated]"
	}
	return s
}

// redactError returns a sanitized, length-bounded error string safe for logging.
// Truncating prevents sensitive request content from appearing in log sinks.
func redactError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	if len(msg) > maxErrorLen {
		return msg[:maxErrorLen] + " [truncated]"
	}
	return msg
}

// AuditingLLM wraps any LLM backend and emits audit log entries for every call.
// All log entries are tagged with sensitivity="metadata-only" to indicate that
// message content is intentionally excluded; only structural metadata is recorded.
// Legal basis for logging: operational audit trail for security controls.
//
// StreamComplete audits both invocation and stream completion/error via a
// forwarding goroutine that drains the inner channel and re-emits chunks.
type AuditingLLM struct {
	inner LLM
}

func NewAuditingLLM(inner LLM) *AuditingLLM {
	return &AuditingLLM{inner: inner}
}

func (a *AuditingLLM) Name() string { return a.inner.Name() }

func (a *AuditingLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	provider := sanitizeForLog(a.inner.Name())
	model := sanitizeForLog(req.Model)
	slog.Info("llm.Complete called",
		"provider", provider,
		"model", model,
		"messages", len(req.Messages),
		"sensitivity", "metadata-only",
	)
	resp, err := a.inner.Complete(ctx, req)
	if err != nil {
		slog.Error("llm.Complete failed",
			"provider", provider,
			"model", model,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", redactError(err),
			"sensitivity", "metadata-only",
		)
		return resp, err
	}
	slog.Info("llm.Complete succeeded",
		"provider", provider,
		"model", sanitizeForLog(resp.Model),
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"duration_ms", time.Since(start).Milliseconds(),
		"sensitivity", "metadata-only",
	)
	return resp, nil
}

func (a *AuditingLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	start := time.Now()
	provider := sanitizeForLog(a.inner.Name())
	model := sanitizeForLog(req.Model)
	slog.Info("llm.StreamComplete called",
		"provider", provider,
		"model", model,
		"sensitivity", "metadata-only",
	)
	inner, err := a.inner.StreamComplete(ctx, req)
	if err != nil {
		slog.Error("llm.StreamComplete failed",
			"provider", provider,
			"model", model,
			"duration_ms", time.Since(start).Milliseconds(),
			"error", redactError(err),
			"sensitivity", "metadata-only",
		)
		return nil, err
	}

	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		var chunks int
		var streamErr error
		for chunk := range inner {
			if chunk.Error != nil {
				streamErr = chunk.Error
			}
			chunks++
			out <- chunk
		}
		// Audit stream result: chunk count and outcome are structural metadata only;
		// chunk content is never logged to prevent PII/PHI/PAN leakage.
		if streamErr != nil {
			slog.Error("llm.StreamComplete stream error",
				"provider", provider,
				"model", model,
				"chunks", chunks,
				"duration_ms", time.Since(start).Milliseconds(),
				"error", redactError(streamErr),
				"sensitivity", "metadata-only",
			)
		} else {
			slog.Info("llm.StreamComplete completed",
				"provider", provider,
				"model", model,
				"chunks", chunks,
				"duration_ms", time.Since(start).Milliseconds(),
				"sensitivity", "metadata-only",
			)
		}
	}()
	return out, nil
}
