package app

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/mandadapu/neuralforge/internal/config"
	"github.com/mandadapu/neuralforge/internal/executor"
	"github.com/mandadapu/neuralforge/internal/git"
	"github.com/mandadapu/neuralforge/internal/github"
	"github.com/mandadapu/neuralforge/internal/llm"
	"github.com/mandadapu/neuralforge/internal/pipeline"
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

// buildJobHandler creates the LLM backend and executor, then returns a handler
// that clones the repo, builds pipeline state, and runs the pipeline engine.
func (a *App) buildJobHandler() worker.JobHandler {
	// Create LLM backend based on config.
	var backend llm.LLM
	switch a.cfg.LLM.DefaultProvider {
	case "openai":
		backend = llm.NewOpenAI(a.cfg.LLM.OpenAI.APIKey, a.cfg.LLM.OpenAI.Model)
	default:
		backend = llm.NewClaude(a.cfg.LLM.Claude.APIKey, a.cfg.LLM.Claude.Model)
	}

	// Create executor based on config.
	var exec executor.Executor
	switch a.cfg.Executor.DefaultType {
	case "kubernetes":
		k8sCfg := a.cfg.Executor.Kubernetes
		var err error
		exec, err = executor.NewKubernetes(
			k8sCfg.Namespace, k8sCfg.Image,
			k8sCfg.SecretName, k8sCfg.GitSecretName,
			k8sCfg.CPU, k8sCfg.Memory,
		)
		if err != nil {
			slog.Error("failed to create k8s executor, falling back to docker", "error", err)
			exec = executor.NewDocker(a.cfg.Executor.Docker.Image)
		}
	default:
		exec = executor.NewDocker(a.cfg.Executor.Docker.Image)
	}

	return func(ctx context.Context, job store.Job) error {
		slog.Info("processing job", "job_id", job.ID, "repo", job.RepoFullName, "issue", job.IssueNumber)

		defer func() {
			if err := exec.Cleanup(ctx, job.ID); err != nil {
				slog.Warn("executor cleanup failed", "job_id", job.ID, "executor", exec.Name(), "error", err)
			}
		}()

		// 1. Create temp dir and clone the repo.
		tmpDir, err := os.MkdirTemp("", "neuralforge-*")
		if err != nil {
			return fmt.Errorf("create temp dir: %w", err)
		}
		defer os.RemoveAll(tmpDir)

		cloneDir := filepath.Join(tmpDir, "repo")
		cloneURL := fmt.Sprintf("https://github.com/%s.git", job.RepoFullName)
		// TODO: pass GitHub App installation token once App auth is implemented.
		if _, err := git.Clone(cloneURL, cloneDir, ""); err != nil {
			return fmt.Errorf("clone repo: %w", err)
		}

		// 2. Build PipelineState from the job.
		state := &pipeline.PipelineState{
			JobID: job.ID,
			Issue: pipeline.GitHubIssue{
				Number: job.IssueNumber,
				Title:  job.IssueTitle,
			},
			Repo: pipeline.RepoContext{
				FullName:      job.RepoFullName,
				DefaultBranch: "main",
				CloneURL:      cloneURL,
				LocalPath:     cloneDir,
			},
		}

		// 3. Create pipeline stages.
		// Wire the first 5 stages; PR/Review/Merge/Deploy require GitHub App auth.
		var execTimeout time.Duration
		if a.cfg.Executor.DefaultType == "kubernetes" {
			execTimeout = a.cfg.Executor.Kubernetes.Timeout
		} else {
			execTimeout = a.cfg.Executor.Docker.Timeout
		}

		stages := []pipeline.Stage{
			pipeline.NewArchitectStage(backend),
			pipeline.NewSecurityStage(backend),
			pipeline.NewExecuteStage(exec, execTimeout),
			pipeline.NewVerifyStage("make test"),
			pipeline.NewComplianceStage(2000, 50),
		}
		// TODO: wire remaining stages once GitHub App authentication is implemented:
		//   pipeline.NewPRStage(ghClient)
		//   pipeline.NewReviewStage(backend, ghClient)
		//   pipeline.NewMergeStage(ghClient)
		//   pipeline.NewDeployStage(ghClient)

		// 4. Create engine with budget from config.
		engine := pipeline.NewEngine(stages, &pipeline.EngineConfig{
			BudgetUSD: 5.0,
		})

		// 5. Run the pipeline.
		if err := engine.Run(ctx, state); err != nil {
			return fmt.Errorf("pipeline run: %w", err)
		}

		slog.Info("job completed", "job_id", job.ID, "cost", state.Cost)
		return nil
	}
}
