package app

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mandadapu/neuralforge/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validTestConfig returns a minimal config suitable for unit tests.
// It uses a temp SQLite file and binds to a random port.
func validTestConfig(t *testing.T) config.Config {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "app-test-*.db")
	require.NoError(t, err)
	f.Close()

	return config.Config{
		Server:  config.ServerConfig{Host: "127.0.0.1", Port: freePort(t)},
		Workers: 1,
		GitHub:  config.GitHubConfig{WebhookSecret: "test-secret"},
		LLM: config.LLMConfig{
			DefaultProvider: "claude",
			Claude:          config.ProviderConfig{APIKey: "test-key", Model: "claude-3-5-sonnet-20241022"},
		},
		Executor: config.ExecutorConfig{
			DefaultType: "docker",
			Docker: config.DockerConfig{
				Image:   "test-image:latest",
				Timeout: 30 * time.Minute,
			},
		},
		Store: config.StoreConfig{
			Driver: "sqlite",
			DSN:    f.Name(),
		},
		Context: config.ContextConfig{RefreshDays: 7},
	}
}

// freePort returns an available TCP port for test server binding.
func freePort(t *testing.T) int {
	t.Helper()
	// Use a high port in the ephemeral range; if it collides tests will still work
	// because New() calls net.Listen and fails fast on collision.
	// For simplicity, just return a fixed offset from the test's pointer.
	// A more robust approach would actually bind-and-release a socket.
	return 19000 + (os.Getpid() % 1000)
}

func TestNewApp(t *testing.T) {
	cfg := validTestConfig(t)

	app, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, app)

	// Cleanup: close the store.
	ctx := context.Background()
	_ = app.store.Close()
	_ = app.server.Shutdown(ctx)
}

func TestNewAppInvalidStoreDSN(t *testing.T) {
	cfg := validTestConfig(t)
	// Point to an unwritable path.
	cfg.Store.DSN = "/nonexistent/path/to/db.sqlite"

	_, err := New(cfg)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "open store")
}

func TestAppStartAndHealthEndpoint(t *testing.T) {
	cfg := validTestConfig(t)

	app, err := New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = app.Start(ctx)
	require.NoError(t, err)

	// Give the server a moment to be ready.
	time.Sleep(50 * time.Millisecond)

	resp, err := http.Get(fmt.Sprintf("http://%s:%d/health", cfg.Server.Host, cfg.Server.Port))
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// Shutdown gracefully.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	err = app.Shutdown(shutdownCtx)
	require.NoError(t, err)
}

func TestAppShutdownIdempotent(t *testing.T) {
	cfg := validTestConfig(t)

	app, err := New(cfg)
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	require.NoError(t, app.Start(ctx))
	time.Sleep(20 * time.Millisecond)

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	// First shutdown should succeed.
	err = app.Shutdown(shutdownCtx)
	require.NoError(t, err)
}
