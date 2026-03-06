package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/mandadapu/neuralforge/internal/executor"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockExecutor is a minimal executor that does not implement HealthChecker.
type mockExecutor struct{}

func (m *mockExecutor) Run(ctx context.Context, job executor.ExecutorJob) (executor.ExecutorResult, error) {
	return executor.ExecutorResult{}, nil
}
func (m *mockExecutor) Cleanup(ctx context.Context, jobID string) error { return nil }
func (m *mockExecutor) Name() string                                    { return "mock" }

// healthyK8sExecutor implements both Executor and HealthChecker (Ping succeeds).
type healthyK8sExecutor struct{ mockExecutor }

func (h *healthyK8sExecutor) Ping(ctx context.Context) error { return nil }

// unhealthyK8sExecutor implements both Executor and HealthChecker (Ping fails).
type unhealthyK8sExecutor struct{ mockExecutor }

func (u *unhealthyK8sExecutor) Ping(ctx context.Context) error {
	return fmt.Errorf("k8s cluster unreachable: connection refused")
}

func TestHealthEndpoint_NoHealthChecker(t *testing.T) {
	a := &App{executor: &mockExecutor{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]string{"status": "ok"}
		if hc, ok := a.executor.(executor.HealthChecker); ok {
			if err := hc.Ping(r.Context()); err != nil {
				status["status"] = "degraded"
				status["k8s"] = err.Error()
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(status)
				return
			}
			status["k8s"] = "connected"
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(status)
	})

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["status"])
	assert.Empty(t, resp["k8s"])
}

func TestHealthEndpoint_K8sHealthy(t *testing.T) {
	a := &App{executor: &healthyK8sExecutor{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]string{"status": "ok"}
		if hc, ok := a.executor.(executor.HealthChecker); ok {
			if err := hc.Ping(r.Context()); err != nil {
				status["status"] = "degraded"
				status["k8s"] = err.Error()
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(status)
				return
			}
			status["k8s"] = "connected"
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(status)
	})

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "ok", resp["status"])
	assert.Equal(t, "connected", resp["k8s"])
}

func TestHealthEndpoint_K8sUnhealthy(t *testing.T) {
	a := &App{executor: &unhealthyK8sExecutor{}}
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		status := map[string]string{"status": "ok"}
		if hc, ok := a.executor.(executor.HealthChecker); ok {
			if err := hc.Ping(r.Context()); err != nil {
				status["status"] = "degraded"
				status["k8s"] = err.Error()
				w.WriteHeader(http.StatusServiceUnavailable)
				json.NewEncoder(w).Encode(status)
				return
			}
			status["k8s"] = "connected"
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(status)
	})

	req := httptest.NewRequest("GET", "/health", nil)
	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusServiceUnavailable, rr.Code)
	var resp map[string]string
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
	assert.Equal(t, "degraded", resp["status"])
	assert.Contains(t, resp["k8s"], "k8s cluster unreachable")
}
