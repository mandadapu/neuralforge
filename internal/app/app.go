package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/mandadapu/neuralforge/internal/config"
	"github.com/mandadapu/neuralforge/internal/github"
	"github.com/mandadapu/neuralforge/internal/store"
	"github.com/mandadapu/neuralforge/internal/worker"
)

// App wires together the store, worker pool, and HTTP server.
type App struct {
	cfg    config.Config
	store  store.Store
	pool   *worker.Pool
	server *http.Server
}

// New creates a new App, opens the SQLite store, and runs migrations.
func New(cfg config.Config) (*App, error) {
	s, err := store.NewSQLiteStore(cfg.Store.DSN)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := s.Migrate(); err != nil {
		s.Close()
		return nil, fmt.Errorf("migrate store: %w", err)
	}

	a := &App{
		cfg:   cfg,
		store: s,
	}

	handler := a.buildJobHandler()
	a.pool = worker.NewPool(cfg.Workers, s, handler)

	mux := http.NewServeMux()
	mux.Handle("/webhooks/github", NewWebhookHandler(cfg.GitHub.WebhookSecret, a.handleEvent))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	})

	a.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler: mux,
	}

	return a, nil
}

// Start starts the worker pool and HTTP server.
func (a *App) Start(ctx context.Context) error {
	if err := a.pool.Start(ctx); err != nil {
		return fmt.Errorf("start worker pool: %w", err)
	}

	slog.Info("starting server", "addr", a.server.Addr)

	ln, err := net.Listen("tcp", a.server.Addr)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	go func() {
		if err := a.server.Serve(ln); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "error", err)
		}
	}()

	return nil
}

// Shutdown gracefully shuts down the HTTP server and closes the store.
func (a *App) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	if err := a.server.Shutdown(shutdownCtx); err != nil {
		slog.Error("server shutdown error", "error", err)
	}

	if err := a.store.Close(); err != nil {
		return fmt.Errorf("close store: %w", err)
	}

	slog.Info("app shut down gracefully")
	return nil
}

// handleEvent parses a webhook event and creates a job for issue labeled events.
func (a *App) handleEvent(eventType string, payload []byte) {
	event, err := github.ParseWebhookEvent(eventType, payload)
	if err != nil {
		slog.Error("parse webhook event", "error", err)
		return
	}
	if event == nil {
		return
	}

	switch e := event.(type) {
	case *github.IssueLabeledEvent:
		if e.Label != "neuralforge" {
			return
		}
		jobID := fmt.Sprintf("%s#%d", e.Repo.FullName, e.Issue.Number)
		job := store.Job{
			ID:           jobID,
			RepoFullName: e.Repo.FullName,
			IssueNumber:  e.Issue.Number,
			IssueTitle:   e.Issue.Title,
			Status:       store.JobQueued,
		}
		if err := a.store.CreateJob(job); err != nil {
			slog.Error("create job", "error", err, "job_id", jobID)
			return
		}
		slog.Info("job created", "job_id", jobID, "issue", e.Issue.Title)
	default:
		slog.Info("unhandled event type", "type", event.EventType())
	}
}

// buildJobHandler returns a placeholder handler that logs job processing.
// Full pipeline wiring will be added in Task 16.
func (a *App) buildJobHandler() worker.JobHandler {
	return func(ctx context.Context, job store.Job) error {
		slog.Info("processing job", "job_id", job.ID, "repo", job.RepoFullName, "issue", job.IssueNumber)
		// TODO(task-16): wire pipeline engine here
		return nil
	}
}
