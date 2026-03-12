package llm

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"time"
)

// auditActorKey is the context key used to propagate an actor/user identifier
// into audit log entries (SOX segregation-of-duties).
type auditActorKey struct{}

// WithActor returns a context that carries the given actor identifier.
// Callers (e.g. job handlers) should inject this before invoking LLM methods.
func WithActor(ctx context.Context, actor string) context.Context {
	return context.WithValue(ctx, auditActorKey{}, actor)
}

// actorFromCtx extracts the actor identifier from the context, or returns
// "unknown" when none has been set.
func actorFromCtx(ctx context.Context) string {
	if v, ok := ctx.Value(auditActorKey{}).(string); ok && v != "" {
		return v
	}
	return "unknown"
}

// promptHash returns a stable SHA-256 hex digest of the request's system
// prompt and messages. This satisfies GDPR Article 30 processing records
// (data minimization) without logging raw prompt content.
func promptHash(req CompletionRequest) string {
	h := sha256.New()
	fmt.Fprint(h, req.System)
	for _, m := range req.Messages {
		fmt.Fprintf(h, "%s:%s", m.Role, m.Content)
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}

// AuditedLLM wraps any LLM and emits structured audit log entries for every call.
type AuditedLLM struct {
	inner LLM
}

// NewAudited wraps inner with audit logging. All callers should obtain LLM
// instances via the llm.New factory rather than constructing backends directly.
func NewAudited(inner LLM) LLM {
	return &AuditedLLM{inner: inner}
}

func (a *AuditedLLM) Name() string { return a.inner.Name() }

func (a *AuditedLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	start := time.Now()
	resp, err := a.inner.Complete(ctx, req)
	attrs := []any{
		"provider", a.inner.Name(),
		"model", resp.Model,
		"input_tokens", resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd", resp.Cost,
		"latency_ms", time.Since(start).Milliseconds(),
		"actor", actorFromCtx(ctx),
		"prompt_hash", promptHash(req),
	}
	if err != nil {
		slog.Error("llm audit: Complete failed", append(attrs, "error", err)...)
	} else {
		slog.Info("llm audit: Complete", attrs...)
	}
	return resp, err
}

// StreamComplete initiates a streaming completion and wraps the returned channel
// so that completion events and errors are captured in the audit log.
func (a *AuditedLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	start := time.Now()
	actor := actorFromCtx(ctx)
	pHash := promptHash(req)

	slog.Info("llm audit: StreamComplete started",
		"provider", a.inner.Name(),
		"model", req.Model,
		"actor", actor,
		"prompt_hash", pHash,
	)

	inner, err := a.inner.StreamComplete(ctx, req)
	if err != nil {
		slog.Error("llm audit: StreamComplete failed",
			"provider", a.inner.Name(),
			"model", req.Model,
			"actor", actor,
			"prompt_hash", pHash,
			"latency_ms", time.Since(start).Milliseconds(),
			"error", err,
		)
		return nil, err
	}

	// Wrap the channel to observe completion and per-chunk errors.
	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		var chunkCount int
		var lastErr error
		for chunk := range inner {
			if chunk.Error != nil {
				lastErr = chunk.Error
			}
			if !chunk.Done {
				chunkCount++
			}
			out <- chunk
		}
		attrs := []any{
			"provider", a.inner.Name(),
			"model", req.Model,
			"actor", actor,
			"prompt_hash", pHash,
			"chunks", chunkCount,
			"latency_ms", time.Since(start).Milliseconds(),
		}
		if lastErr != nil {
			slog.Error("llm audit: StreamComplete completed with error", append(attrs, "error", lastErr)...)
		} else {
			slog.Info("llm audit: StreamComplete completed", attrs...)
		}
	}()

	return out, nil
}
