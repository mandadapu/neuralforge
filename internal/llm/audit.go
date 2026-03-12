package llm

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
)

// AuditingLLM wraps any LLM and emits a structured audit log entry
// for every completion request (provider, model, token usage, cost).
type AuditingLLM struct {
	inner    LLM
	endpoint string // API endpoint for GDPR data-transfer audit
	region   string // geographic region of the API endpoint for GDPR transfer mechanism verification
}

func (a *AuditingLLM) Name() string { return a.inner.Name() }

// requestHash returns a short SHA-256 hash of the request content for audit
// correlation without logging raw payloads (which may contain PHI/PII).
func requestHash(req CompletionRequest) string {
	h := sha256.New()
	h.Write([]byte(req.System))
	for _, m := range req.Messages {
		h.Write([]byte(m.Role))
		h.Write([]byte(m.Content))
	}
	return hex.EncodeToString(h.Sum(nil))[:16]
}

func (a *AuditingLLM) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	reqHash := requestHash(req)
	resp, err := a.inner.Complete(ctx, req)
	if err != nil {
		slog.Error("llm completion failed",
			"provider", a.inner.Name(),
			"endpoint", a.endpoint,
			"region",   a.region,
			"req_hash", reqHash,
			"error", err,
		)
		return resp, err
	}
	slog.Info("llm completion",
		"provider",      a.inner.Name(),
		"endpoint",      a.endpoint,
		"region",        a.region,
		"model",         resp.Model,
		"input_tokens",  resp.InputTokens,
		"output_tokens", resp.OutputTokens,
		"cost_usd",      resp.Cost,
		"req_hash",      reqHash,
	)
	return resp, nil
}

func (a *AuditingLLM) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	reqHash := requestHash(req)
	slog.Info("llm stream_complete start",
		"provider", a.inner.Name(),
		"endpoint", a.endpoint,
		"region",   a.region,
		"req_hash", reqHash,
	)
	ch, err := a.inner.StreamComplete(ctx, req)
	if err != nil {
		slog.Error("llm stream_complete failed",
			"provider", a.inner.Name(),
			"endpoint", a.endpoint,
			"region",   a.region,
			"req_hash", reqHash,
			"error", err,
		)
		return nil, err
	}
	out := make(chan StreamChunk)
	go func() {
		defer close(out)
		var chunkCount, errorCount int
		for chunk := range ch {
			if chunk.Error != nil {
				errorCount++
				slog.Error("llm stream_complete chunk error",
					"provider",    a.inner.Name(),
					"endpoint",    a.endpoint,
					"region",      a.region,
					"req_hash",    reqHash,
					"error_count", errorCount,
					"error",       chunk.Error,
				)
			}
			chunkCount++
			out <- chunk
		}
		slog.Info("llm stream_complete done",
			"provider",    a.inner.Name(),
			"endpoint",    a.endpoint,
			"region",      a.region,
			"req_hash",    reqHash,
			"chunk_count", chunkCount,
			"error_count", errorCount,
		)
	}()
	return out, nil
}

// NewFactory creates the appropriate LLM backend for the given provider and wraps
// it in an AuditingLLM so every call is audit-logged through a single path.
func NewFactory(provider, apiKey, model string) (LLM, error) {
	var inner LLM
	var endpoint string
	var region string
	switch provider {
	case "openai":
		inner = NewOpenAI(apiKey, model)
		endpoint = "api.openai.com"
		region = "us-east-1" // OpenAI US datacenter; relevant for GDPR transfer mechanism
	case "claude":
		inner = NewClaude(apiKey, model)
		endpoint = "api.anthropic.com"
		region = "us-east-1" // Anthropic US datacenter; relevant for GDPR transfer mechanism
	case "":
		return nil, fmt.Errorf("LLM provider must be specified; got empty string")
	default:
		return nil, fmt.Errorf("unknown LLM provider: %q", provider)
	}
	return &AuditingLLM{inner: inner, endpoint: endpoint, region: region}, nil
}
