package llm

// AuditedLLM wraps an inner LLM and logs audit events for every call.
//
// Privacy/Legal basis for logging:
//   - Only metadata is logged (provider, model, token counts, cost, latency, error type).
//   - Message content is NEVER logged; only the message count is recorded.
//   - Error message content is NEVER logged; only the error type name is recorded.
//   - Callers must ensure that model names and provider names do not themselves
//     constitute personal data before passing them through.
//   - This logging constitutes legitimate interest under GDPR Art. 6(1)(f) for
//     operational security and financial audit purposes (SOX, HIPAA access logs).

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// sanitizeError returns only the reflect type name of err for audit logging.
// Error message content is deliberately excluded to prevent leakage of sensitive
// API response data (API keys, auth tokens, PII/PHI) into log streams.
// The full error value is preserved and returned to callers unmodified.
func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("%T", err)
}

type AuditedLLM struct {
	inner  LLM
	logger *slog.Logger
}

// NewAuditedLLM returns an LLM that delegates to inner and logs audit events.
// If logger is nil, slog.Default() is used.
func NewAuditedLLM(inner LLM, logger *slog.Logger) LLM {
	if logger == nil {
		logger = slog.Default()
	}
	return &AuditedLLM{inner: inner, logger: logger}
}

func (a *AuditedLLM) Name() string {
	return "audited/" + a.inner.Name()
}

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	// Log only the message count — content is never recorded to protect PII/PHI.
	a.logger.Info("llm.call.start", "provider", a.inner.Name(), "model", req.Model, "message_count", len(req.Messages))
	resp, err := a.inner.Complete(ctx, req)
	elapsed := time.Since(start)
	if err != nil {
		a.logger.Error("llm.call.error", "provider", a.inner.Name(), "model", req.Model, "duration_ms", elapsed.Milliseconds(), "error", sanitizeError(err))
		return resp, err
	}
	a.logger.Info("llm.call.done", "provider", a.inner.Name(), "model", resp.Model,
		"input_tokens", resp.InputTokens, "output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost, "duration_ms", elapsed.Milliseconds())
	return resp, nil
}

// StreamComplete logs start and end audit events (including errors and duration)
// around the streamed response. Token counts are not available from stream chunks,
// so output_chars is recorded as a proxy.
func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	start := time.Now()
	// Log only the message count — content is never recorded to protect PII/PHI.
	a.logger.Info("llm.stream.start", "provider", a.inner.Name(), "model", req.Model, "message_count", len(req.Messages))

	innerCh, err := a.inner.StreamComplete(ctx, req)
	if err != nil {
		elapsed := time.Since(start)
		a.logger.Error("llm.stream.error", "provider", a.inner.Name(), "model", req.Model,
			"duration_ms", elapsed.Milliseconds(), "error", sanitizeError(err))
		return nil, err
	}

	outCh := make(chan StreamChunk)
	go func() {
		defer close(outCh)
		var outputChars int
		for chunk := range innerCh {
			outputChars += len(chunk.Content)
			outCh <- chunk
			if chunk.Error != nil {
				elapsed := time.Since(start)
				a.logger.Error("llm.stream.error", "provider", a.inner.Name(), "model", req.Model,
					"duration_ms", elapsed.Milliseconds(), "error", sanitizeError(chunk.Error))
				return
			}
			if chunk.Done {
				break
			}
		}
		elapsed := time.Since(start)
		// Token counts and cost are not available from stream chunks; -1 signals
		// "not available" for SOX/HIPAA audit consumers. output_chars is a proxy.
		a.logger.Info("llm.stream.done", "provider", a.inner.Name(), "model", req.Model,
			"output_chars", outputChars,
			"input_tokens", -1, "output_tokens", -1, "cost_usd", -1.0,
			"duration_ms", elapsed.Milliseconds())
	}()
	return outCh, nil
}
