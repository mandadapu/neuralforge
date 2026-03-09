package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/mandadapu/neuralforge/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testConfig(t *testing.T) config.Config {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	return config.Config{
		Server:  config.ServerConfig{Port: 0, Host: "127.0.0.1"},
		Workers: 1,
		GitHub: config.GitHubConfig{
			AppID:         0, // no GitHub App auth
			WebhookSecret: "test-secret",
		},
		LLM: config.LLMConfig{
			DefaultProvider: "claude",
			Claude:          config.ProviderConfig{APIKey: "sk-test", Model: "claude-sonnet-4-5-20250514"},
		},
		Executor: config.ExecutorConfig{
			DefaultType: "docker",
			Docker:      config.DockerConfig{Image: "test:latest", Timeout: 30 * time.Minute},
		},
		Store: config.StoreConfig{
			Driver: "sqlite",
			DSN:    dbPath,
		},
		Context: config.ContextConfig{
			RefreshDays: 7,
		},
	}
}

func TestNew_CreatesAppSuccessfully(t *testing.T) {
	cfg := testConfig(t)

	app, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, app)

	// Clean up store
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = app.Shutdown(ctx)
}

func TestNew_BadDSN_ReturnsError(t *testing.T) {
	cfg := testConfig(t)
	cfg.Store.DSN = "/nonexistent/path/db.sqlite"

	_, err := New(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open store")
}

func TestApp_HealthEndpoint(t *testing.T) {
	cfg := testConfig(t)

	app, err := New(cfg)
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = app.Shutdown(ctx)
	}()

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rr := httptest.NewRecorder()
	app.server.Handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "ok")
}

func TestApp_HandleEvent_NeuralforgeLabel_CreatesJob(t *testing.T) {
	cfg := testConfig(t)

	app, err := New(cfg)
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = app.Shutdown(ctx)
	}()

	payload := []byte(`{
		"action": "labeled",
		"label": {"name": "neuralforge"},
		"issue": {
			"number": 99,
			"title": "Test issue",
			"body": "test body",
			"user": {"login": "alice"},
			"labels": [{"name": "neuralforge"}]
		},
		"repository": {
			"full_name": "owner/testrepo",
			"default_branch": "main",
			"clone_url": "https://github.com/owner/testrepo.git"
		}
	}`)

	app.handleEvent("issues", payload)

	// Verify the job was created in the store
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	job, err := app.store.GetJobByIssue(ctx, "owner/testrepo", 99)
	require.NoError(t, err)
	require.NotNil(t, job)
	assert.Equal(t, "owner/testrepo#99", job.ID)
	assert.Equal(t, "Test issue", job.IssueTitle)
}

func TestApp_HandleEvent_OtherLabel_DoesNotCreateJob(t *testing.T) {
	cfg := testConfig(t)

	app, err := New(cfg)
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = app.Shutdown(ctx)
	}()

	payload := []byte(`{
		"action": "labeled",
		"label": {"name": "bug"},
		"issue": {
			"number": 100,
			"title": "Some issue",
			"body": "body",
			"user": {"login": "alice"},
			"labels": [{"name": "bug"}]
		},
		"repository": {
			"full_name": "owner/testrepo",
			"default_branch": "main",
			"clone_url": "https://github.com/owner/testrepo.git"
		}
	}`)

	app.handleEvent("issues", payload)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	job, err := app.store.GetJobByIssue(ctx, "owner/testrepo", 100)
	require.NoError(t, err)
	assert.Nil(t, job, "job should not be created for non-neuralforge labels")
}

func TestApp_HandleEvent_UnknownEventType_NoError(t *testing.T) {
	cfg := testConfig(t)

	app, err := New(cfg)
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = app.Shutdown(ctx)
	}()

	// Should not panic
	app.handleEvent("push", []byte(`{"ref":"refs/heads/main"}`))
}

func TestApp_HandleEvent_InvalidPayload_NoError(t *testing.T) {
	cfg := testConfig(t)

	app, err := New(cfg)
	require.NoError(t, err)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = app.Shutdown(ctx)
	}()

	// Invalid JSON should not panic
	app.handleEvent("issues", []byte(`{invalid json`))
}
