# NeuralForge v1 Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Build a standalone Go binary that installs as a GitHub App, watches for labeled issues, and autonomously produces reviewed, tested PRs through a 10-stage pipeline with 5 parallel workers.

**Architecture:** Webhook receiver enqueues jobs into a buffered channel backed by SQLite. Worker pool pulls jobs, clones repos, ensures CLAUDE.md context, runs the full pipeline (architect → security → execute → verify → compliance → PR → review → merge → deploy), and reports results back to GitHub.

**Tech Stack:** Go 1.22+, cobra (CLI), chi (HTTP router), go-github (GitHub API), modernc.org/sqlite (CGo-free SQLite), anthropic-sdk-go, openai-go

---

## Phase 1: Project Scaffolding & Core Types (Tasks 1-3)

### Task 1: Project Scaffolding

**Files:**
- Create: `go.mod`
- Create: `cmd/neuralforge/main.go`
- Create: `Makefile`
- Create: `.gitignore`
- Create: `CLAUDE.md`

**Step 1: Initialize Go module**

```bash
cd ~/src/neuralforge
go mod init github.com/mandadapu/neuralforge
```

**Step 2: Create entry point**

Create `cmd/neuralforge/main.go`:
```go
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("neuralforge v0.1.0-dev")
	return nil
}
```

**Step 3: Create Makefile**

```makefile
.PHONY: build test lint clean

BINARY := neuralforge
VERSION := 0.1.0-dev

build:
	go build -ldflags "-X main.version=$(VERSION)" -o bin/$(BINARY) ./cmd/neuralforge

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf bin/
```

**Step 4: Create .gitignore**

```
bin/
*.db
*.pem
.env
vendor/
```

**Step 5: Create CLAUDE.md**

```markdown
# NeuralForge

Go binary — autonomous software factory. GitHub App that watches issues and produces PRs.

## Build & Test
- `make build` — compile to `bin/neuralforge`
- `make test` — run all tests with race detector
- `go test ./internal/... -v` — verbose tests

## Structure
- `cmd/neuralforge/` — CLI entry point (cobra)
- `internal/` — all packages (config, llm, executor, pipeline, store, git, github, worker, context)

## Conventions
- Standard Go project layout
- Interfaces in dedicated files (e.g., `llm.go`, `executor.go`, `store.go`)
- Table-driven tests, `testify/assert` for assertions
- Error wrapping with `fmt.Errorf("context: %w", err)`
```

**Step 6: Build and verify**

```bash
go build ./cmd/neuralforge
```
Expected: compiles with no errors

**Step 7: Commit**

```bash
git add -A && git commit -m "chore: scaffold Go project with entry point and Makefile"
```

---

### Task 2: Core Interfaces & Types

**Files:**
- Create: `internal/llm/llm.go`
- Create: `internal/executor/executor.go`
- Create: `internal/pipeline/stage.go`
- Create: `internal/pipeline/state.go`

**Step 1: Create LLM interface**

Create `internal/llm/llm.go`:
```go
package llm

import "context"

type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

type Message struct {
	Role    Role   `json:"role"`
	Content string `json:"content"`
}

type CompletionRequest struct {
	System      string    `json:"system"`
	Messages    []Message `json:"messages"`
	Model       string    `json:"model"`
	MaxTokens   int       `json:"max_tokens"`
	Temperature float64   `json:"temperature"`
}

type CompletionResponse struct {
	Content      string `json:"content"`
	Model        string `json:"model"`
	InputTokens  int    `json:"input_tokens"`
	OutputTokens int    `json:"output_tokens"`
	Cost         float64
}

type StreamChunk struct {
	Content string
	Done    bool
	Error   error
}

type LLM interface {
	Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
	StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
	Name() string
}
```

**Step 2: Create Executor interface**

Create `internal/executor/executor.go`:
```go
package executor

import (
	"context"
	"time"
)

type ExecutorJob struct {
	ID       string
	RepoPath string
	Branch   string
	Prompt   string
	Context  string
	Timeout  time.Duration
	EnvVars  map[string]string
}

type ExecutorResult struct {
	Success      bool
	Stdout       string
	Stderr       string
	FilesChanged []string
	TimedOut     bool
}

type Executor interface {
	Run(ctx context.Context, job ExecutorJob) (ExecutorResult, error)
	Cleanup(ctx context.Context, jobID string) error
	Name() string
}
```

**Step 3: Create Stage interface**

Create `internal/pipeline/stage.go`:
```go
package pipeline

import "context"

type StageStatus string

const (
	StatusPassed  StageStatus = "passed"
	StatusFailed  StageStatus = "failed"
	StatusSkipped StageStatus = "skipped"
)

type StageResult struct {
	Status  StageStatus
	Output  string
	Details map[string]any
}

type Stage interface {
	Name() string
	Run(ctx context.Context, state *PipelineState) (StageResult, error)
}
```

**Step 4: Create PipelineState**

Create `internal/pipeline/state.go`:
```go
package pipeline

import "time"

type GitHubIssue struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Labels []string `json:"labels"`
	Author string `json:"author"`
}

type RepoContext struct {
	FullName     string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	CloneURL     string `json:"clone_url"`
	LocalPath    string `json:"local_path"`
}

type FileChange struct {
	Path   string `json:"path"`
	Action string `json:"action"` // added, modified, deleted
	Diff   string `json:"diff"`
}

type TestReport struct {
	Passed  bool   `json:"passed"`
	Output  string `json:"output"`
	Command string `json:"command"`
}

type ComplianceReport struct {
	Passed       bool     `json:"passed"`
	DiffLines    int      `json:"diff_lines"`
	FilesChanged int      `json:"files_changed"`
	Violations   []string `json:"violations"`
}

type StageLog struct {
	Name      string      `json:"name"`
	Status    StageStatus `json:"status"`
	Output    string      `json:"output"`
	Duration  time.Duration `json:"duration"`
	StartedAt time.Time   `json:"started_at"`
}

type PipelineState struct {
	JobID         string          `json:"job_id"`
	Issue         GitHubIssue     `json:"issue"`
	Repo          RepoContext     `json:"repo"`
	Memory        string          `json:"memory"`
	Plan          string          `json:"plan"`
	SecurityNotes string          `json:"security_notes"`
	Changes       []FileChange    `json:"changes"`
	TestResults   *TestReport     `json:"test_results"`
	Compliance    *ComplianceReport `json:"compliance"`
	PRURL         string          `json:"pr_url"`
	PRNumber      int             `json:"pr_number"`
	ReviewNotes   string          `json:"review_notes"`
	Merged        bool            `json:"merged"`
	DeployURL     string          `json:"deploy_url"`
	StartedAt     time.Time       `json:"started_at"`
	Cost          float64         `json:"cost"`
	Stages        []StageLog      `json:"stages"`
}
```

**Step 5: Verify compilation**

```bash
go build ./...
```

**Step 6: Commit**

```bash
git add internal/ && git commit -m "feat: add core interfaces — LLM, Executor, Stage, PipelineState"
```

---

### Task 3: Configuration

**Files:**
- Create: `internal/config/config.go`
- Create: `internal/config/config_test.go`
- Create: `internal/config/repoconfig.go`
- Create: `internal/config/repoconfig_test.go`

**Step 1: Write config test**

Create `internal/config/config_test.go`:
```go
package config

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("NEURALFORGE_PORT", "9090")
	t.Setenv("NEURALFORGE_WORKERS", "3")
	t.Setenv("GITHUB_APP_ID", "12345")
	t.Setenv("GITHUB_WEBHOOK_SECRET", "test-secret")
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test")

	cfg := LoadFromEnv()

	assert.Equal(t, 9090, cfg.Server.Port)
	assert.Equal(t, 3, cfg.Workers)
	assert.Equal(t, int64(12345), cfg.GitHub.AppID)
	assert.Equal(t, "test-secret", cfg.GitHub.WebhookSecret)
	assert.Equal(t, "sk-ant-test", cfg.LLM.Claude.APIKey)
}

func TestLoadFromEnvDefaults(t *testing.T) {
	// Clear relevant env vars
	for _, k := range []string{"NEURALFORGE_PORT", "NEURALFORGE_WORKERS"} {
		os.Unsetenv(k)
	}

	cfg := LoadFromEnv()

	assert.Equal(t, 8080, cfg.Server.Port)
	assert.Equal(t, 5, cfg.Workers)
	assert.Equal(t, "sqlite", cfg.Store.Driver)
}
```

**Step 2: Run test to verify it fails**

```bash
cd ~/src/neuralforge && go test ./internal/config/ -v
```
Expected: FAIL — package doesn't exist yet

**Step 3: Implement config**

Create `internal/config/config.go`:
```go
package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Workers  int
	GitHub   GitHubConfig
	LLM      LLMConfig
	Executor ExecutorConfig
	Store    StoreConfig
	Context  ContextConfig
}

type ServerConfig struct {
	Port int
	Host string
}

type GitHubConfig struct {
	AppID          int64
	PrivateKeyPath string
	WebhookSecret  string
}

type LLMConfig struct {
	DefaultProvider string
	Claude          ProviderConfig
	OpenAI          ProviderConfig
}

type ProviderConfig struct {
	APIKey string
	Model  string
}

type ExecutorConfig struct {
	DefaultType string
	Docker      DockerConfig
}

type DockerConfig struct {
	Image   string
	Timeout time.Duration
}

type StoreConfig struct {
	Driver string
	DSN    string
}

type ContextConfig struct {
	AutoGenerate  bool
	RefreshDays   int
	AnalysisDepth string
	CommitToRepo  bool
}

func LoadFromEnv() Config {
	return Config{
		Server: ServerConfig{
			Port: envInt("NEURALFORGE_PORT", 8080),
			Host: envStr("NEURALFORGE_HOST", "0.0.0.0"),
		},
		Workers: envInt("NEURALFORGE_WORKERS", 5),
		GitHub: GitHubConfig{
			AppID:          int64(envInt("GITHUB_APP_ID", 0)),
			PrivateKeyPath: envStr("GITHUB_PRIVATE_KEY_PATH", ""),
			WebhookSecret:  envStr("GITHUB_WEBHOOK_SECRET", ""),
		},
		LLM: LLMConfig{
			DefaultProvider: envStr("NEURALFORGE_LLM_PROVIDER", "claude"),
			Claude: ProviderConfig{
				APIKey: envStr("ANTHROPIC_API_KEY", ""),
				Model:  envStr("ANTHROPIC_MODEL", "claude-sonnet-4-5-20250514"),
			},
			OpenAI: ProviderConfig{
				APIKey: envStr("OPENAI_API_KEY", ""),
				Model:  envStr("OPENAI_MODEL", "gpt-4o"),
			},
		},
		Executor: ExecutorConfig{
			DefaultType: envStr("NEURALFORGE_EXECUTOR", "docker"),
			Docker: DockerConfig{
				Image:   envStr("NEURALFORGE_DOCKER_IMAGE", "ghcr.io/neuralforge/executor:latest"),
				Timeout: time.Duration(envInt("NEURALFORGE_TIMEOUT_MINUTES", 30)) * time.Minute,
			},
		},
		Store: StoreConfig{
			Driver: envStr("NEURALFORGE_STORE_DRIVER", "sqlite"),
			DSN:    envStr("NEURALFORGE_STORE_DSN", "neuralforge.db"),
		},
		Context: ContextConfig{
			AutoGenerate:  envBool("NEURALFORGE_AUTO_CONTEXT", true),
			RefreshDays:   envInt("NEURALFORGE_CONTEXT_REFRESH_DAYS", 7),
			AnalysisDepth: envStr("NEURALFORGE_ANALYSIS_DEPTH", "thorough"),
			CommitToRepo:  envBool("NEURALFORGE_CONTEXT_COMMIT", true),
		},
	}
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return fallback
}
```

**Step 4: Run test to verify it passes**

```bash
go test ./internal/config/ -v
```
Expected: PASS

**Step 5: Write repo config test**

Create `internal/config/repoconfig_test.go`:
```go
package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRepoConfig(t *testing.T) {
	yaml := `
neuralforge:
  trigger:
    label: "autofix"
  llm:
    provider: claude
    model: claude-sonnet-4-5-20250514
  executor:
    type: docker
  pipeline:
    architecture_review: true
    security_review: true
    code_review: true
    auto_merge: false
  limits:
    max_files_changed: 50
    timeout_minutes: 30
    budget_usd: 5.0
`
	cfg, err := ParseRepoConfig([]byte(yaml))
	require.NoError(t, err)

	assert.Equal(t, "autofix", cfg.Trigger.Label)
	assert.Equal(t, "claude", cfg.LLM.Provider)
	assert.Equal(t, true, cfg.Pipeline.ArchitectureReview)
	assert.Equal(t, false, cfg.Pipeline.AutoMerge)
	assert.Equal(t, 50, cfg.Limits.MaxFilesChanged)
	assert.InDelta(t, 5.0, cfg.Limits.BudgetUSD, 0.01)
}

func TestParseRepoConfigDefaults(t *testing.T) {
	cfg, err := ParseRepoConfig([]byte(""))
	require.NoError(t, err)

	assert.Equal(t, "neuralforge", cfg.Trigger.Label)
	assert.Equal(t, 5.0, cfg.Limits.BudgetUSD)
}
```

**Step 6: Implement repo config**

Create `internal/config/repoconfig.go`:
```go
package config

import "gopkg.in/yaml.v3"

type RepoConfig struct {
	Trigger  TriggerConfig  `yaml:"trigger"`
	LLM      RepoLLMConfig  `yaml:"llm"`
	Executor RepoExecConfig `yaml:"executor"`
	Pipeline PipelineConfig `yaml:"pipeline"`
	Limits   LimitsConfig   `yaml:"limits"`
}

type TriggerConfig struct {
	Label string `yaml:"label"`
}

type RepoLLMConfig struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

type RepoExecConfig struct {
	Type string `yaml:"type"`
}

type PipelineConfig struct {
	ArchitectureReview bool              `yaml:"architecture_review"`
	SecurityReview     bool              `yaml:"security_review"`
	Verification       VerificationConfig `yaml:"verification"`
	Compliance         ComplianceConfig  `yaml:"compliance"`
	CodeReview         bool              `yaml:"code_review"`
	AutoMerge          bool              `yaml:"auto_merge"`
	CIDeploy           bool              `yaml:"ci_deploy"`
}

type VerificationConfig struct {
	Command string `yaml:"command"`
}

type ComplianceConfig struct {
	MaxDiffLines    int      `yaml:"max_diff_lines"`
	BlockedLicenses []string `yaml:"blocked_licenses"`
}

type LimitsConfig struct {
	MaxFilesChanged int     `yaml:"max_files_changed"`
	TimeoutMinutes  int     `yaml:"timeout_minutes"`
	BudgetUSD       float64 `yaml:"budget_usd"`
}

type repoConfigWrapper struct {
	NeuralForge RepoConfig `yaml:"neuralforge"`
}

func ParseRepoConfig(data []byte) (RepoConfig, error) {
	cfg := RepoConfig{
		Trigger: TriggerConfig{Label: "neuralforge"},
		Limits: LimitsConfig{
			MaxFilesChanged: 50,
			TimeoutMinutes:  30,
			BudgetUSD:       5.0,
		},
	}

	if len(data) == 0 {
		return cfg, nil
	}

	var wrapper repoConfigWrapper
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return cfg, err
	}

	merged := wrapper.NeuralForge
	if merged.Trigger.Label == "" {
		merged.Trigger.Label = cfg.Trigger.Label
	}
	if merged.Limits.BudgetUSD == 0 {
		merged.Limits.BudgetUSD = cfg.Limits.BudgetUSD
	}
	if merged.Limits.MaxFilesChanged == 0 {
		merged.Limits.MaxFilesChanged = cfg.Limits.MaxFilesChanged
	}
	if merged.Limits.TimeoutMinutes == 0 {
		merged.Limits.TimeoutMinutes = cfg.Limits.TimeoutMinutes
	}

	return merged, nil
}
```

**Step 7: Install deps and run tests**

```bash
go get gopkg.in/yaml.v3 github.com/stretchr/testify
go test ./internal/config/ -v
```
Expected: PASS

**Step 8: Commit**

```bash
git add -A && git commit -m "feat: add global config (env-based) and per-repo .neuralforge.yml parser"
```

---

## Phase 2: Persistence & Git (Tasks 4-5)

### Task 4: SQLite Store

**Files:**
- Create: `internal/store/store.go`
- Create: `internal/store/sqlite.go`
- Create: `internal/store/sqlite_test.go`

**Step 1: Write store interface**

Create `internal/store/store.go`:
```go
package store

import "time"

type JobStatus string

const (
	JobQueued     JobStatus = "queued"
	JobRunning    JobStatus = "running"
	JobCompleted  JobStatus = "completed"
	JobFailed     JobStatus = "failed"
)

type Job struct {
	ID            string    `json:"id"`
	RepoFullName  string    `json:"repo_full_name"`
	IssueNumber   int       `json:"issue_number"`
	IssueTitle    string    `json:"issue_title"`
	Status        JobStatus `json:"status"`
	CurrentStage  string    `json:"current_stage"`
	PipelineState string    `json:"pipeline_state"`
	Error         string    `json:"error"`
	CostUSD       float64   `json:"cost_usd"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	CompletedAt   *time.Time `json:"completed_at"`
}

type RepoContextRecord struct {
	RepoFullName   string    `json:"repo_full_name"`
	ClaudeMDHash   string    `json:"claude_md_hash"`
	LastAnalyzedAt time.Time `json:"last_analyzed_at"`
	FileCount      int       `json:"file_count"`
	Languages      []string  `json:"languages"`
}

type Store interface {
	// Jobs
	CreateJob(job Job) error
	GetJob(id string) (*Job, error)
	GetJobByIssue(repoFullName string, issueNumber int) (*Job, error)
	UpdateJobStatus(id string, status JobStatus, stage string) error
	UpdateJobError(id string, errMsg string) error
	UpdateJobCost(id string, cost float64) error
	CompleteJob(id string, status JobStatus) error
	ListPendingJobs(limit int) ([]Job, error)

	// Repo contexts
	UpsertRepoContext(ctx RepoContextRecord) error
	GetRepoContext(repoFullName string) (*RepoContextRecord, error)

	// Lifecycle
	Migrate() error
	Close() error
}
```

**Step 2: Write SQLite tests**

Create `internal/store/sqlite_test.go`:
```go
package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	s, err := NewSQLiteStore(":memory:")
	require.NoError(t, err)
	require.NoError(t, s.Migrate())
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndGetJob(t *testing.T) {
	s := newTestStore(t)

	job := Job{
		ID:           "job-1",
		RepoFullName: "owner/repo",
		IssueNumber:  42,
		IssueTitle:   "Fix bug",
		Status:       JobQueued,
	}
	require.NoError(t, s.CreateJob(job))

	got, err := s.GetJob("job-1")
	require.NoError(t, err)
	assert.Equal(t, "owner/repo", got.RepoFullName)
	assert.Equal(t, 42, got.IssueNumber)
	assert.Equal(t, JobQueued, got.Status)
}

func TestGetJobByIssue(t *testing.T) {
	s := newTestStore(t)

	job := Job{ID: "job-2", RepoFullName: "owner/repo", IssueNumber: 7, Status: JobQueued}
	require.NoError(t, s.CreateJob(job))

	got, err := s.GetJobByIssue("owner/repo", 7)
	require.NoError(t, err)
	assert.Equal(t, "job-2", got.ID)
}

func TestUpdateJobStatus(t *testing.T) {
	s := newTestStore(t)

	job := Job{ID: "job-3", RepoFullName: "o/r", IssueNumber: 1, Status: JobQueued}
	require.NoError(t, s.CreateJob(job))
	require.NoError(t, s.UpdateJobStatus("job-3", JobRunning, "architect"))

	got, err := s.GetJob("job-3")
	require.NoError(t, err)
	assert.Equal(t, JobRunning, got.Status)
	assert.Equal(t, "architect", got.CurrentStage)
}

func TestListPendingJobs(t *testing.T) {
	s := newTestStore(t)

	require.NoError(t, s.CreateJob(Job{ID: "j1", RepoFullName: "o/r", IssueNumber: 1, Status: JobQueued}))
	require.NoError(t, s.CreateJob(Job{ID: "j2", RepoFullName: "o/r", IssueNumber: 2, Status: JobRunning}))
	require.NoError(t, s.CreateJob(Job{ID: "j3", RepoFullName: "o/r", IssueNumber: 3, Status: JobCompleted}))

	jobs, err := s.ListPendingJobs(10)
	require.NoError(t, err)
	assert.Len(t, jobs, 1)
	assert.Equal(t, "j1", jobs[0].ID)
}

func TestCompleteJob(t *testing.T) {
	s := newTestStore(t)

	require.NoError(t, s.CreateJob(Job{ID: "j4", RepoFullName: "o/r", IssueNumber: 4, Status: JobRunning}))
	require.NoError(t, s.CompleteJob("j4", JobCompleted))

	got, err := s.GetJob("j4")
	require.NoError(t, err)
	assert.Equal(t, JobCompleted, got.Status)
	assert.NotNil(t, got.CompletedAt)
}
```

**Step 3: Run tests to verify they fail**

```bash
go test ./internal/store/ -v
```
Expected: FAIL

**Step 4: Implement SQLite store**

Create `internal/store/sqlite.go`:
```go
package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore(dsn string) (*SQLiteStore, error) {
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// WAL mode for concurrent writes
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		return nil, fmt.Errorf("set WAL mode: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

func (s *SQLiteStore) Migrate() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS jobs (
			id TEXT PRIMARY KEY,
			repo_full_name TEXT NOT NULL,
			issue_number INTEGER NOT NULL,
			issue_title TEXT DEFAULT '',
			status TEXT DEFAULT 'queued',
			current_stage TEXT DEFAULT '',
			pipeline_state TEXT DEFAULT '{}',
			error TEXT DEFAULT '',
			cost_usd REAL DEFAULT 0,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			completed_at TIMESTAMP,
			UNIQUE(repo_full_name, issue_number)
		);
		CREATE TABLE IF NOT EXISTS repo_contexts (
			repo_full_name TEXT PRIMARY KEY,
			claude_md_hash TEXT DEFAULT '',
			last_analyzed_at TIMESTAMP,
			file_count INTEGER DEFAULT 0,
			languages TEXT DEFAULT '[]',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	return err
}

func (s *SQLiteStore) Close() error {
	return s.db.Close()
}

func (s *SQLiteStore) CreateJob(job Job) error {
	_, err := s.db.Exec(
		`INSERT INTO jobs (id, repo_full_name, issue_number, issue_title, status, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		job.ID, job.RepoFullName, job.IssueNumber, job.IssueTitle, job.Status,
		time.Now(), time.Now(),
	)
	return err
}

func (s *SQLiteStore) GetJob(id string) (*Job, error) {
	row := s.db.QueryRow(
		`SELECT id, repo_full_name, issue_number, issue_title, status, current_stage,
		        pipeline_state, error, cost_usd, created_at, updated_at, completed_at
		 FROM jobs WHERE id = ?`, id,
	)
	return scanJob(row)
}

func (s *SQLiteStore) GetJobByIssue(repoFullName string, issueNumber int) (*Job, error) {
	row := s.db.QueryRow(
		`SELECT id, repo_full_name, issue_number, issue_title, status, current_stage,
		        pipeline_state, error, cost_usd, created_at, updated_at, completed_at
		 FROM jobs WHERE repo_full_name = ? AND issue_number = ?`,
		repoFullName, issueNumber,
	)
	return scanJob(row)
}

func (s *SQLiteStore) UpdateJobStatus(id string, status JobStatus, stage string) error {
	_, err := s.db.Exec(
		`UPDATE jobs SET status = ?, current_stage = ?, updated_at = ? WHERE id = ?`,
		status, stage, time.Now(), id,
	)
	return err
}

func (s *SQLiteStore) UpdateJobError(id string, errMsg string) error {
	_, err := s.db.Exec(
		`UPDATE jobs SET error = ?, status = ?, updated_at = ? WHERE id = ?`,
		errMsg, JobFailed, time.Now(), id,
	)
	return err
}

func (s *SQLiteStore) UpdateJobCost(id string, cost float64) error {
	_, err := s.db.Exec(
		`UPDATE jobs SET cost_usd = ?, updated_at = ? WHERE id = ?`,
		cost, time.Now(), id,
	)
	return err
}

func (s *SQLiteStore) CompleteJob(id string, status JobStatus) error {
	now := time.Now()
	_, err := s.db.Exec(
		`UPDATE jobs SET status = ?, completed_at = ?, updated_at = ? WHERE id = ?`,
		status, now, now, id,
	)
	return err
}

func (s *SQLiteStore) ListPendingJobs(limit int) ([]Job, error) {
	rows, err := s.db.Query(
		`SELECT id, repo_full_name, issue_number, issue_title, status, current_stage,
		        pipeline_state, error, cost_usd, created_at, updated_at, completed_at
		 FROM jobs WHERE status = ? ORDER BY created_at ASC LIMIT ?`,
		JobQueued, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []Job
	for rows.Next() {
		j, err := scanJobRow(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, *j)
	}
	return jobs, rows.Err()
}

func (s *SQLiteStore) UpsertRepoContext(ctx RepoContextRecord) error {
	langs, _ := json.Marshal(ctx.Languages)
	_, err := s.db.Exec(
		`INSERT INTO repo_contexts (repo_full_name, claude_md_hash, last_analyzed_at, file_count, languages)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(repo_full_name) DO UPDATE SET
		   claude_md_hash = excluded.claude_md_hash,
		   last_analyzed_at = excluded.last_analyzed_at,
		   file_count = excluded.file_count,
		   languages = excluded.languages`,
		ctx.RepoFullName, ctx.ClaudeMDHash, ctx.LastAnalyzedAt, ctx.FileCount, string(langs),
	)
	return err
}

func (s *SQLiteStore) GetRepoContext(repoFullName string) (*RepoContextRecord, error) {
	var rc RepoContextRecord
	var langs string
	err := s.db.QueryRow(
		`SELECT repo_full_name, claude_md_hash, last_analyzed_at, file_count, languages
		 FROM repo_contexts WHERE repo_full_name = ?`, repoFullName,
	).Scan(&rc.RepoFullName, &rc.ClaudeMDHash, &rc.LastAnalyzedAt, &rc.FileCount, &langs)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	json.Unmarshal([]byte(langs), &rc.Languages)
	return &rc, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanJob(row *sql.Row) (*Job, error) {
	var j Job
	var completedAt sql.NullTime
	err := row.Scan(
		&j.ID, &j.RepoFullName, &j.IssueNumber, &j.IssueTitle,
		&j.Status, &j.CurrentStage, &j.PipelineState, &j.Error,
		&j.CostUSD, &j.CreatedAt, &j.UpdatedAt, &completedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		j.CompletedAt = &completedAt.Time
	}
	return &j, nil
}

func scanJobRow(rows *sql.Rows) (*Job, error) {
	var j Job
	var completedAt sql.NullTime
	err := rows.Scan(
		&j.ID, &j.RepoFullName, &j.IssueNumber, &j.IssueTitle,
		&j.Status, &j.CurrentStage, &j.PipelineState, &j.Error,
		&j.CostUSD, &j.CreatedAt, &j.UpdatedAt, &completedAt,
	)
	if err != nil {
		return nil, err
	}
	if completedAt.Valid {
		j.CompletedAt = &completedAt.Time
	}
	return &j, nil
}
```

**Step 5: Install dep and run tests**

```bash
go get modernc.org/sqlite
go test ./internal/store/ -v
```
Expected: PASS

**Step 6: Commit**

```bash
git add -A && git commit -m "feat: add SQLite store with jobs and repo_contexts tables"
```

---

### Task 5: Git Operations

**Files:**
- Create: `internal/git/git.go`
- Create: `internal/git/git_test.go`

**Step 1: Write git operations test**

Create `internal/git/git_test.go`:
```go
package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		require.NoError(t, exec.Command(args[0], args[1:]...).Run())
	}
	// Create initial commit
	f := filepath.Join(dir, "README.md")
	require.NoError(t, os.WriteFile(f, []byte("# Test"), 0644))
	require.NoError(t, exec.Command("git", "-C", dir, "add", ".").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "-m", "init").Run())
	return dir
}

func TestCreateBranch(t *testing.T) {
	repo := initTestRepo(t)
	g := New(repo)

	err := g.CreateBranch("feature-1")
	require.NoError(t, err)

	branch, err := g.CurrentBranch()
	require.NoError(t, err)
	assert.Equal(t, "feature-1", branch)
}

func TestCommitAndLog(t *testing.T) {
	repo := initTestRepo(t)
	g := New(repo)

	require.NoError(t, g.CreateBranch("test-branch"))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "new.txt"), []byte("hello"), 0644))
	require.NoError(t, g.AddAll())
	require.NoError(t, g.Commit("add new file"))

	log, err := g.Log(1)
	require.NoError(t, err)
	assert.Contains(t, log, "add new file")
}

func TestDiffStat(t *testing.T) {
	repo := initTestRepo(t)
	g := New(repo)

	require.NoError(t, g.CreateBranch("diff-branch"))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "a.txt"), []byte("content"), 0644))
	require.NoError(t, g.AddAll())
	require.NoError(t, g.Commit("add a"))

	stat, err := g.DiffStat("master")
	require.NoError(t, err)
	assert.Contains(t, stat, "a.txt")
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/git/ -v
```

**Step 3: Implement git operations**

Create `internal/git/git.go`:
```go
package git

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

type Git struct {
	dir string
}

func New(dir string) *Git {
	return &Git{dir: dir}
}

func Clone(url, dir, token string) (*Git, error) {
	cloneURL := url
	if token != "" {
		cloneURL = strings.Replace(url, "https://", fmt.Sprintf("https://x-access-token:%s@", token), 1)
	}
	if err := run("git", "clone", "--depth=1", cloneURL, dir); err != nil {
		return nil, fmt.Errorf("clone: %w", err)
	}
	return New(dir), nil
}

func (g *Git) run(args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", g.dir}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s: %w", args[0], stderr.String(), err)
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (g *Git) CreateBranch(name string) error {
	_, err := g.run("checkout", "-b", name)
	return err
}

func (g *Git) Checkout(name string) error {
	_, err := g.run("checkout", name)
	return err
}

func (g *Git) CurrentBranch() (string, error) {
	return g.run("rev-parse", "--abbrev-ref", "HEAD")
}

func (g *Git) AddAll() error {
	_, err := g.run("add", "-A")
	return err
}

func (g *Git) Commit(message string) error {
	_, err := g.run("commit", "-m", message)
	return err
}

func (g *Git) Push(remote, branch string) error {
	_, err := g.run("push", remote, branch)
	return err
}

func (g *Git) PushNewBranch(remote, branch string) error {
	_, err := g.run("push", "-u", remote, branch)
	return err
}

func (g *Git) Log(n int) (string, error) {
	return g.run("log", fmt.Sprintf("-%d", n), "--oneline")
}

func (g *Git) DiffStat(base string) (string, error) {
	return g.run("diff", "--stat", base)
}

func (g *Git) DiffLines(base string) (int, error) {
	out, err := g.run("diff", "--shortstat", base)
	if err != nil {
		return 0, err
	}
	// Parse "N files changed, X insertions(+), Y deletions(-)"
	lines := 0
	for _, part := range strings.Split(out, ",") {
		part = strings.TrimSpace(part)
		var n int
		if strings.Contains(part, "insertion") {
			fmt.Sscanf(part, "%d", &n)
			lines += n
		} else if strings.Contains(part, "deletion") {
			fmt.Sscanf(part, "%d", &n)
			lines += n
		}
	}
	return lines, nil
}

func (g *Git) FilesChanged(base string) ([]string, error) {
	out, err := g.run("diff", "--name-only", base)
	if err != nil {
		return nil, err
	}
	if out == "" {
		return nil, nil
	}
	return strings.Split(out, "\n"), nil
}

func (g *Git) Dir() string {
	return g.dir
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %s: %w", name, args[0], stderr.String(), err)
	}
	return nil
}
```

**Step 4: Run tests**

```bash
go test ./internal/git/ -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: add git operations wrapper (clone, branch, commit, diff)"
```

---

## Phase 3: GitHub Client & LLM Backends (Tasks 6-8)

### Task 6: GitHub Client

**Files:**
- Create: `internal/github/client.go`
- Create: `internal/github/events.go`
- Create: `internal/github/events_test.go`

**Step 1: Write webhook parsing test**

Create `internal/github/events_test.go`:
```go
package github

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIssueLabeled(t *testing.T) {
	payload := []byte(`{
		"action": "labeled",
		"label": {"name": "neuralforge"},
		"issue": {
			"number": 42,
			"title": "Fix login bug",
			"body": "The login page crashes on submit",
			"user": {"login": "alice"},
			"labels": [{"name": "bug"}, {"name": "neuralforge"}]
		},
		"repository": {
			"full_name": "owner/repo",
			"default_branch": "main",
			"clone_url": "https://github.com/owner/repo.git"
		}
	}`)

	evt, err := ParseWebhookEvent("issues", payload)
	require.NoError(t, err)

	ie, ok := evt.(*IssueLabeledEvent)
	require.True(t, ok)
	assert.Equal(t, "neuralforge", ie.Label)
	assert.Equal(t, 42, ie.Issue.Number)
	assert.Equal(t, "owner/repo", ie.Repo.FullName)
}

func TestParseIssueComment(t *testing.T) {
	payload := []byte(`{
		"action": "created",
		"comment": {"body": "/retry", "user": {"login": "bob"}},
		"issue": {"number": 42, "title": "Fix it"},
		"repository": {"full_name": "owner/repo", "default_branch": "main", "clone_url": "https://github.com/owner/repo.git"}
	}`)

	evt, err := ParseWebhookEvent("issue_comment", payload)
	require.NoError(t, err)

	ce, ok := evt.(*IssueCommentEvent)
	require.True(t, ok)
	assert.Equal(t, "/retry", ce.Command)
	assert.Equal(t, 42, ce.Issue.Number)
}

func TestParseUnknownEvent(t *testing.T) {
	evt, err := ParseWebhookEvent("push", []byte(`{}`))
	require.NoError(t, err)
	assert.Nil(t, evt)
}
```

**Step 2: Implement events**

Create `internal/github/events.go`:
```go
package github

import (
	"encoding/json"
	"strings"

	"github.com/mandadapu/neuralforge/internal/pipeline"
)

type Event interface {
	EventType() string
}

type IssueLabeledEvent struct {
	Label string
	Issue pipeline.GitHubIssue
	Repo  pipeline.RepoContext
}

func (e *IssueLabeledEvent) EventType() string { return "issues.labeled" }

type IssueCommentEvent struct {
	Command string
	Author  string
	Issue   pipeline.GitHubIssue
	Repo    pipeline.RepoContext
}

func (e *IssueCommentEvent) EventType() string { return "issue_comment.created" }

func ParseWebhookEvent(eventType string, payload []byte) (Event, error) {
	switch eventType {
	case "issues":
		return parseIssueEvent(payload)
	case "issue_comment":
		return parseCommentEvent(payload)
	default:
		return nil, nil
	}
}

func parseIssueEvent(payload []byte) (Event, error) {
	var raw struct {
		Action string `json:"action"`
		Label  struct {
			Name string `json:"name"`
		} `json:"label"`
		Issue struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
			Body   string `json:"body"`
			User   struct {
				Login string `json:"login"`
			} `json:"user"`
			Labels []struct {
				Name string `json:"name"`
			} `json:"labels"`
		} `json:"issue"`
		Repo rawRepo `json:"repository"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	if raw.Action != "labeled" {
		return nil, nil
	}
	labels := make([]string, len(raw.Issue.Labels))
	for i, l := range raw.Issue.Labels {
		labels[i] = l.Name
	}
	return &IssueLabeledEvent{
		Label: raw.Label.Name,
		Issue: pipeline.GitHubIssue{
			Number: raw.Issue.Number,
			Title:  raw.Issue.Title,
			Body:   raw.Issue.Body,
			Labels: labels,
			Author: raw.Issue.User.Login,
		},
		Repo: repoContext(raw.Repo),
	}, nil
}

func parseCommentEvent(payload []byte) (Event, error) {
	var raw struct {
		Action  string `json:"action"`
		Comment struct {
			Body string `json:"body"`
			User struct {
				Login string `json:"login"`
			} `json:"user"`
		} `json:"comment"`
		Issue struct {
			Number int    `json:"number"`
			Title  string `json:"title"`
		} `json:"issue"`
		Repo rawRepo `json:"repository"`
	}
	if err := json.Unmarshal(payload, &raw); err != nil {
		return nil, err
	}
	if raw.Action != "created" {
		return nil, nil
	}
	body := strings.TrimSpace(raw.Comment.Body)
	if !strings.HasPrefix(body, "/") {
		return nil, nil
	}
	return &IssueCommentEvent{
		Command: strings.Fields(body)[0],
		Author:  raw.Comment.User.Login,
		Issue: pipeline.GitHubIssue{
			Number: raw.Issue.Number,
			Title:  raw.Issue.Title,
		},
		Repo: repoContext(raw.Repo),
	}, nil
}

type rawRepo struct {
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	CloneURL      string `json:"clone_url"`
}

func repoContext(r rawRepo) pipeline.RepoContext {
	return pipeline.RepoContext{
		FullName:      r.FullName,
		DefaultBranch: r.DefaultBranch,
		CloneURL:      r.CloneURL,
	}
}
```

**Step 3: Create GitHub client (API wrapper)**

Create `internal/github/client.go`:
```go
package github

import (
	"context"
	"fmt"
	"net/http"

	gh "github.com/google/go-github/v60/github"
)

type Client struct {
	gh *gh.Client
}

func NewClient(httpClient *http.Client) *Client {
	return &Client{gh: gh.NewClient(httpClient)}
}

func (c *Client) CreatePR(ctx context.Context, owner, repo, title, body, head, base string) (int, string, error) {
	pr, _, err := c.gh.PullRequests.Create(ctx, owner, repo, &gh.NewPullRequest{
		Title: gh.Ptr(title),
		Body:  gh.Ptr(body),
		Head:  gh.Ptr(head),
		Base:  gh.Ptr(base),
	})
	if err != nil {
		return 0, "", fmt.Errorf("create PR: %w", err)
	}
	return pr.GetNumber(), pr.GetHTMLURL(), nil
}

func (c *Client) CommentOnIssue(ctx context.Context, owner, repo string, number int, body string) error {
	_, _, err := c.gh.Issues.CreateComment(ctx, owner, repo, number, &gh.IssueComment{
		Body: gh.Ptr(body),
	})
	return err
}

func (c *Client) MergePR(ctx context.Context, owner, repo string, number int, message string) error {
	_, _, err := c.gh.PullRequests.Merge(ctx, owner, repo, number, message, nil)
	return err
}

func (c *Client) CreateReview(ctx context.Context, owner, repo string, prNumber int, body string, event string) error {
	_, _, err := c.gh.PullRequests.CreateReview(ctx, owner, repo, prNumber, &gh.PullRequestReviewRequest{
		Body:  gh.Ptr(body),
		Event: gh.Ptr(event),
	})
	return err
}

func (c *Client) GetFileContent(ctx context.Context, owner, repo, path, ref string) (string, error) {
	fc, _, _, err := c.gh.Repositories.GetContents(ctx, owner, repo, path, &gh.RepositoryContentGetOptions{Ref: ref})
	if err != nil {
		return "", err
	}
	content, err := fc.GetContent()
	return content, err
}
```

**Step 4: Run tests**

```bash
go get github.com/google/go-github/v60
go test ./internal/github/ -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: add GitHub webhook parser and API client"
```

---

### Task 7: Claude LLM Backend

**Files:**
- Create: `internal/llm/claude.go`
- Create: `internal/llm/claude_test.go`
- Create: `internal/llm/cost.go`

**Step 1: Write cost tracking test**

Create `internal/llm/cost.go`:
```go
package llm

// Cost per million tokens (USD) — updated as pricing changes
var modelCosts = map[string]struct{ input, output float64 }{
	"claude-sonnet-4-5-20250514": {3.0, 15.0},
	"claude-opus-4-6":            {15.0, 75.0},
	"claude-haiku-4-5-20251001":  {0.80, 4.0},
	"gpt-4o":                     {2.50, 10.0},
	"gpt-4o-mini":                {0.15, 0.60},
}

func CalculateCost(model string, inputTokens, outputTokens int) float64 {
	costs, ok := modelCosts[model]
	if !ok {
		// Default conservative estimate
		costs = struct{ input, output float64 }{3.0, 15.0}
	}
	return (float64(inputTokens)/1_000_000)*costs.input +
		(float64(outputTokens)/1_000_000)*costs.output
}
```

Create `internal/llm/claude_test.go`:
```go
package llm

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		model  string
		input  int
		output int
		want   float64
	}{
		{"claude-sonnet-4-5-20250514", 1000, 500, 0.000003 + 0.0000075},
		{"gpt-4o", 1_000_000, 0, 2.50},
		{"unknown-model", 1000, 1000, 0.000003 + 0.000015},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := CalculateCost(tt.model, tt.input, tt.output)
			assert.InDelta(t, tt.want, got, 0.0001)
		})
	}
}
```

**Step 2: Implement Claude backend**

Create `internal/llm/claude.go`:
```go
package llm

import (
	"context"
	"fmt"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

type ClaudeBackend struct {
	client       anthropic.Client
	defaultModel string
}

func NewClaude(apiKey, model string) *ClaudeBackend {
	client := anthropic.NewClient(option.WithAPIKey(apiKey))
	if model == "" {
		model = "claude-sonnet-4-5-20250514"
	}
	return &ClaudeBackend{client: client, defaultModel: model}
}

func (c *ClaudeBackend) Name() string { return "claude" }

func (c *ClaudeBackend) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = c.defaultModel
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	msgs := make([]anthropic.MessageParam, len(req.Messages))
	for i, m := range req.Messages {
		switch m.Role {
		case RoleUser:
			msgs[i] = anthropic.NewUserMessage(anthropic.NewTextBlock(m.Content))
		case RoleAssistant:
			msgs[i] = anthropic.NewAssistantMessage(anthropic.NewTextBlock(m.Content))
		}
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.F(model),
		MaxTokens: anthropic.F(int64(maxTokens)),
		Messages:  anthropic.F(msgs),
	}
	if req.System != "" {
		params.System = anthropic.F([]anthropic.TextBlockParam{
			anthropic.NewTextBlock(req.System),
		})
	}
	if req.Temperature > 0 {
		params.Temperature = anthropic.F(req.Temperature)
	}

	resp, err := c.client.Messages.New(ctx, params)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("claude complete: %w", err)
	}

	var content string
	for _, block := range resp.Content {
		if block.Type == anthropic.ContentBlockTypeText {
			content += block.Text
		}
	}

	inputTokens := int(resp.Usage.InputTokens)
	outputTokens := int(resp.Usage.OutputTokens)

	return CompletionResponse{
		Content:      content,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         CalculateCost(model, inputTokens, outputTokens),
	}, nil
}

func (c *ClaudeBackend) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	// Streaming not needed for v1 pipeline — use Complete
	return nil, fmt.Errorf("streaming not implemented for claude backend")
}
```

**Step 3: Run tests**

```bash
go get github.com/anthropics/anthropic-sdk-go
go test ./internal/llm/ -v
```
Expected: PASS (cost test)

**Step 4: Commit**

```bash
git add -A && git commit -m "feat: add Claude LLM backend with cost tracking"
```

---

### Task 8: OpenAI LLM Backend

**Files:**
- Create: `internal/llm/openai.go`

**Step 1: Implement OpenAI backend**

Create `internal/llm/openai.go`:
```go
package llm

import (
	"context"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type OpenAIBackend struct {
	client       openai.Client
	defaultModel string
}

func NewOpenAI(apiKey, model string) *OpenAIBackend {
	client := openai.NewClient(option.WithAPIKey(apiKey))
	if model == "" {
		model = "gpt-4o"
	}
	return &OpenAIBackend{client: client, defaultModel: model}
}

func (o *OpenAIBackend) Name() string { return "openai" }

func (o *OpenAIBackend) Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = o.defaultModel
	}
	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}

	msgs := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages)+1)
	if req.System != "" {
		msgs = append(msgs, openai.SystemMessage(req.System))
	}
	for _, m := range req.Messages {
		switch m.Role {
		case RoleUser:
			msgs = append(msgs, openai.UserMessage(m.Content))
		case RoleAssistant:
			msgs = append(msgs, openai.AssistantMessage(m.Content))
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:     openai.F(model),
		Messages:  openai.F(msgs),
		MaxTokens: openai.F(int64(maxTokens)),
	}
	if req.Temperature > 0 {
		params.Temperature = openai.F(req.Temperature)
	}

	resp, err := o.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return CompletionResponse{}, fmt.Errorf("openai complete: %w", err)
	}

	var content string
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	inputTokens := int(resp.Usage.PromptTokens)
	outputTokens := int(resp.Usage.CompletionTokens)

	return CompletionResponse{
		Content:      content,
		Model:        model,
		InputTokens:  inputTokens,
		OutputTokens: outputTokens,
		Cost:         CalculateCost(model, inputTokens, outputTokens),
	}, nil
}

func (o *OpenAIBackend) StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error) {
	return nil, fmt.Errorf("streaming not implemented for openai backend")
}
```

**Step 2: Install dep and verify**

```bash
go get github.com/openai/openai-go
go build ./...
```

**Step 3: Commit**

```bash
git add -A && git commit -m "feat: add OpenAI LLM backend"
```

---

## Phase 4: Pipeline Engine & Stages (Tasks 9-11)

### Task 9: Pipeline Engine

**Files:**
- Create: `internal/pipeline/engine.go`
- Create: `internal/pipeline/engine_test.go`

**Step 1: Write engine test**

Create `internal/pipeline/engine_test.go`:
```go
package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStage struct {
	name   string
	result StageResult
	err    error
}

func (m *mockStage) Name() string { return m.name }
func (m *mockStage) Run(_ context.Context, _ *PipelineState) (StageResult, error) {
	return m.result, m.err
}

func TestEngineRunsAllStages(t *testing.T) {
	stages := []Stage{
		&mockStage{name: "s1", result: StageResult{Status: StatusPassed, Output: "ok1"}},
		&mockStage{name: "s2", result: StageResult{Status: StatusPassed, Output: "ok2"}},
	}
	e := NewEngine(stages, nil)

	state := &PipelineState{JobID: "test-1"}
	err := e.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Len(t, state.Stages, 2)
	assert.Equal(t, StatusPassed, state.Stages[0].Status)
}

func TestEngineStopsOnFailure(t *testing.T) {
	stages := []Stage{
		&mockStage{name: "s1", result: StageResult{Status: StatusFailed, Output: "bad"}},
		&mockStage{name: "s2", result: StageResult{Status: StatusPassed}},
	}
	e := NewEngine(stages, nil)

	state := &PipelineState{JobID: "test-2"}
	err := e.Run(context.Background(), state)
	require.Error(t, err)
	assert.Len(t, state.Stages, 1)
	assert.Contains(t, err.Error(), "stage s1 failed")
}

func TestEngineStopsOnError(t *testing.T) {
	stages := []Stage{
		&mockStage{name: "s1", err: errors.New("boom")},
		&mockStage{name: "s2", result: StageResult{Status: StatusPassed}},
	}
	e := NewEngine(stages, nil)

	state := &PipelineState{JobID: "test-3"}
	err := e.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestEngineSkipsStage(t *testing.T) {
	stages := []Stage{
		&mockStage{name: "s1", result: StageResult{Status: StatusSkipped}},
		&mockStage{name: "s2", result: StageResult{Status: StatusPassed}},
	}
	e := NewEngine(stages, nil)

	state := &PipelineState{JobID: "test-4"}
	err := e.Run(context.Background(), state)
	require.NoError(t, err)
	assert.Len(t, state.Stages, 2)
	assert.Equal(t, StatusSkipped, state.Stages[0].Status)
	assert.Equal(t, StatusPassed, state.Stages[1].Status)
}

func TestEngineBudgetExceeded(t *testing.T) {
	stages := []Stage{
		&mockStage{name: "s1", result: StageResult{Status: StatusPassed}},
	}
	e := NewEngine(stages, &EngineConfig{BudgetUSD: 1.0})

	state := &PipelineState{JobID: "test-5", Cost: 1.50}
	err := e.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "budget exceeded")
}
```

**Step 2: Run to verify failure**

```bash
go test ./internal/pipeline/ -v
```

**Step 3: Implement engine**

Create `internal/pipeline/engine.go`:
```go
package pipeline

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

type EngineConfig struct {
	BudgetUSD float64
}

type StageCallback func(state *PipelineState, stage string, status StageStatus)

type Engine struct {
	stages   []Stage
	config   *EngineConfig
	callback StageCallback
}

func NewEngine(stages []Stage, config *EngineConfig) *Engine {
	if config == nil {
		config = &EngineConfig{BudgetUSD: 5.0}
	}
	return &Engine{stages: stages, config: config}
}

func (e *Engine) OnStageComplete(cb StageCallback) {
	e.callback = cb
}

func (e *Engine) Run(ctx context.Context, state *PipelineState) error {
	if state.StartedAt.IsZero() {
		state.StartedAt = time.Now()
	}

	for _, stage := range e.stages {
		// Budget check before each stage
		if e.config.BudgetUSD > 0 && state.Cost > e.config.BudgetUSD {
			return fmt.Errorf("budget exceeded: $%.2f > $%.2f limit", state.Cost, e.config.BudgetUSD)
		}

		slog.Info("running stage", "job", state.JobID, "stage", stage.Name())
		start := time.Now()

		result, err := stage.Run(ctx, state)
		duration := time.Since(start)

		log := StageLog{
			Name:      stage.Name(),
			Duration:  duration,
			StartedAt: start,
		}

		if err != nil {
			log.Status = StatusFailed
			log.Output = err.Error()
			state.Stages = append(state.Stages, log)
			return fmt.Errorf("stage %s error: %w", stage.Name(), err)
		}

		log.Status = result.Status
		log.Output = result.Output
		state.Stages = append(state.Stages, log)

		if e.callback != nil {
			e.callback(state, stage.Name(), result.Status)
		}

		if result.Status == StatusFailed {
			return fmt.Errorf("stage %s failed: %s", stage.Name(), result.Output)
		}
		// StatusSkipped — continue to next stage
	}

	return nil
}
```

**Step 4: Run tests**

```bash
go test ./internal/pipeline/ -v
```
Expected: PASS

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: add pipeline engine with budget checks and stage callbacks"
```

---

### Task 10: Pipeline Stages (Architect, Security, Execute, Verify, Compliance)

**Files:**
- Create: `internal/pipeline/architect.go`
- Create: `internal/pipeline/security.go`
- Create: `internal/pipeline/execute.go`
- Create: `internal/pipeline/verify.go`
- Create: `internal/pipeline/compliance.go`

**Step 1: Implement architect stage**

Create `internal/pipeline/architect.go`:
```go
package pipeline

import (
	"context"
	"fmt"

	"github.com/mandadapu/neuralforge/internal/llm"
)

type ArchitectStage struct {
	llm llm.LLM
}

func NewArchitectStage(l llm.LLM) *ArchitectStage {
	return &ArchitectStage{llm: l}
}

func (s *ArchitectStage) Name() string { return "architect" }

func (s *ArchitectStage) Run(ctx context.Context, state *PipelineState) (StageResult, error) {
	prompt := fmt.Sprintf(`You are a senior software architect. Given this GitHub issue, create a detailed implementation plan.

## Issue #%d: %s

%s

## Codebase Context

%s

## Instructions

Create a step-by-step implementation plan that includes:
1. Which files need to be created or modified
2. The specific changes for each file
3. Test cases that should be written
4. Any potential risks or edge cases

Be specific and actionable. Reference exact file paths from the codebase context.`,
		state.Issue.Number, state.Issue.Title, state.Issue.Body, state.Memory)

	resp, err := s.llm.Complete(ctx, llm.CompletionRequest{
		System:    "You are a senior software architect. Output a clear, actionable implementation plan.",
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		MaxTokens: 4096,
	})
	if err != nil {
		return StageResult{}, fmt.Errorf("architect LLM call: %w", err)
	}

	state.Plan = resp.Content
	state.Cost += resp.Cost

	return StageResult{
		Status: StatusPassed,
		Output: fmt.Sprintf("Plan generated (%d tokens)", resp.OutputTokens),
	}, nil
}
```

**Step 2: Implement security stage**

Create `internal/pipeline/security.go`:
```go
package pipeline

import (
	"context"
	"fmt"

	"github.com/mandadapu/neuralforge/internal/llm"
)

type SecurityStage struct {
	llm llm.LLM
}

func NewSecurityStage(l llm.LLM) *SecurityStage {
	return &SecurityStage{llm: l}
}

func (s *SecurityStage) Name() string { return "security" }

func (s *SecurityStage) Run(ctx context.Context, state *PipelineState) (StageResult, error) {
	if state.Plan == "" {
		return StageResult{Status: StatusSkipped, Output: "no plan to review"}, nil
	}

	prompt := fmt.Sprintf(`Review this implementation plan for security risks:

## Plan
%s

## Codebase Context
%s

Check for:
1. Injection vulnerabilities (SQL, command, XSS)
2. Authentication/authorization issues
3. Sensitive data exposure
4. Insecure dependencies
5. OWASP Top 10 concerns

If the plan is safe, respond with "APPROVED" followed by any minor notes.
If there are critical issues, respond with "BLOCKED" followed by the issues that must be fixed.`,
		state.Plan, state.Memory)

	resp, err := s.llm.Complete(ctx, llm.CompletionRequest{
		System:    "You are a security engineer reviewing an implementation plan.",
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		MaxTokens: 2048,
	})
	if err != nil {
		return StageResult{}, fmt.Errorf("security LLM call: %w", err)
	}

	state.SecurityNotes = resp.Content
	state.Cost += resp.Cost

	return StageResult{
		Status: StatusPassed,
		Output: fmt.Sprintf("Security review complete (%d tokens)", resp.OutputTokens),
	}, nil
}
```

**Step 3: Implement execute stage**

Create `internal/pipeline/execute.go`:
```go
package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/mandadapu/neuralforge/internal/executor"
)

type ExecuteStage struct {
	exec    executor.Executor
	timeout time.Duration
}

func NewExecuteStage(exec executor.Executor, timeout time.Duration) *ExecuteStage {
	if timeout == 0 {
		timeout = 30 * time.Minute
	}
	return &ExecuteStage{exec: exec, timeout: timeout}
}

func (s *ExecuteStage) Name() string { return "execute" }

func (s *ExecuteStage) Run(ctx context.Context, state *PipelineState) (StageResult, error) {
	prompt := fmt.Sprintf(`Implement the following plan for issue #%d: %s

## Plan
%s

## Security Notes
%s

Work in the repository at the given path. Create/modify files as specified in the plan.
Run tests after implementation. Commit your changes.`,
		state.Issue.Number, state.Issue.Title, state.Plan, state.SecurityNotes)

	job := executor.ExecutorJob{
		ID:       state.JobID,
		RepoPath: state.Repo.LocalPath,
		Branch:   fmt.Sprintf("neuralforge/issue-%d", state.Issue.Number),
		Prompt:   prompt,
		Context:  state.Memory,
		Timeout:  s.timeout,
	}

	result, err := s.exec.Run(ctx, job)
	if err != nil {
		return StageResult{}, fmt.Errorf("executor run: %w", err)
	}

	if result.TimedOut {
		return StageResult{
			Status: StatusFailed,
			Output: "Executor timed out",
		}, nil
	}

	if !result.Success {
		return StageResult{
			Status: StatusFailed,
			Output: fmt.Sprintf("Executor failed: %s", result.Stderr),
		}, nil
	}

	state.Changes = make([]FileChange, len(result.FilesChanged))
	for i, f := range result.FilesChanged {
		state.Changes[i] = FileChange{Path: f, Action: "modified"}
	}

	return StageResult{
		Status: StatusPassed,
		Output: fmt.Sprintf("Implementation complete: %d files changed", len(result.FilesChanged)),
	}, nil
}
```

**Step 4: Implement verify stage**

Create `internal/pipeline/verify.go`:
```go
package pipeline

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type VerifyStage struct {
	command string
}

func NewVerifyStage(command string) *VerifyStage {
	if command == "" {
		command = "make test"
	}
	return &VerifyStage{command: command}
}

func (s *VerifyStage) Name() string { return "verify" }

func (s *VerifyStage) Run(ctx context.Context, state *PipelineState) (StageResult, error) {
	if state.Repo.LocalPath == "" {
		return StageResult{Status: StatusSkipped, Output: "no local path"}, nil
	}

	parts := strings.Fields(s.command)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = state.Repo.LocalPath

	output, err := cmd.CombinedOutput()
	passed := err == nil

	state.TestResults = &TestReport{
		Passed:  passed,
		Output:  string(output),
		Command: s.command,
	}

	if !passed {
		return StageResult{
			Status: StatusFailed,
			Output: fmt.Sprintf("Tests failed: %s", string(output)),
		}, nil
	}

	return StageResult{
		Status: StatusPassed,
		Output: "All tests passed",
	}, nil
}
```

**Step 5: Implement compliance stage**

Create `internal/pipeline/compliance.go`:
```go
package pipeline

import (
	"context"
	"fmt"

	gitpkg "github.com/mandadapu/neuralforge/internal/git"
)

type ComplianceStage struct {
	maxDiffLines    int
	maxFilesChanged int
}

func NewComplianceStage(maxDiffLines, maxFilesChanged int) *ComplianceStage {
	if maxDiffLines == 0 {
		maxDiffLines = 2000
	}
	if maxFilesChanged == 0 {
		maxFilesChanged = 50
	}
	return &ComplianceStage{maxDiffLines: maxDiffLines, maxFilesChanged: maxFilesChanged}
}

func (s *ComplianceStage) Name() string { return "compliance" }

func (s *ComplianceStage) Run(ctx context.Context, state *PipelineState) (StageResult, error) {
	if state.Repo.LocalPath == "" {
		return StageResult{Status: StatusSkipped, Output: "no local path"}, nil
	}

	g := gitpkg.New(state.Repo.LocalPath)

	files, err := g.FilesChanged(state.Repo.DefaultBranch)
	if err != nil {
		return StageResult{}, fmt.Errorf("compliance diff: %w", err)
	}

	diffLines, err := g.DiffLines(state.Repo.DefaultBranch)
	if err != nil {
		return StageResult{}, fmt.Errorf("compliance diff lines: %w", err)
	}

	var violations []string
	if len(files) > s.maxFilesChanged {
		violations = append(violations, fmt.Sprintf("too many files changed: %d > %d", len(files), s.maxFilesChanged))
	}
	if diffLines > s.maxDiffLines {
		violations = append(violations, fmt.Sprintf("diff too large: %d lines > %d", diffLines, s.maxDiffLines))
	}

	state.Compliance = &ComplianceReport{
		Passed:       len(violations) == 0,
		DiffLines:    diffLines,
		FilesChanged: len(files),
		Violations:   violations,
	}

	if len(violations) > 0 {
		return StageResult{
			Status: StatusFailed,
			Output: fmt.Sprintf("Compliance failed: %v", violations),
		}, nil
	}

	return StageResult{
		Status: StatusPassed,
		Output: fmt.Sprintf("Compliance passed: %d files, %d diff lines", len(files), diffLines),
	}, nil
}
```

**Step 6: Build and verify**

```bash
go build ./...
```

**Step 7: Commit**

```bash
git add -A && git commit -m "feat: add pipeline stages — architect, security, execute, verify, compliance"
```

---

### Task 11: Pipeline Stages (PR, Review, Merge, Deploy)

**Files:**
- Create: `internal/pipeline/pr.go`
- Create: `internal/pipeline/review.go`
- Create: `internal/pipeline/merge.go`
- Create: `internal/pipeline/deploy.go`

**Step 1: Implement PR stage**

Create `internal/pipeline/pr.go`:
```go
package pipeline

import (
	"context"
	"fmt"
	"strings"

	gh "github.com/mandadapu/neuralforge/internal/github"
)

type PRStage struct {
	client *gh.Client
}

func NewPRStage(client *gh.Client) *PRStage {
	return &PRStage{client: client}
}

func (s *PRStage) Name() string { return "pr" }

func (s *PRStage) Run(ctx context.Context, state *PipelineState) (StageResult, error) {
	parts := strings.SplitN(state.Repo.FullName, "/", 2)
	if len(parts) != 2 {
		return StageResult{}, fmt.Errorf("invalid repo name: %s", state.Repo.FullName)
	}
	owner, repo := parts[0], parts[1]

	title := fmt.Sprintf("fix: #%d %s", state.Issue.Number, state.Issue.Title)
	if len(title) > 72 {
		title = title[:72]
	}

	body := fmt.Sprintf(`## Summary

Automated fix for #%d

## Plan

%s

## Cost

$%.4f

---
Generated by NeuralForge`, state.Issue.Number, state.Plan, state.Cost)

	branch := fmt.Sprintf("neuralforge/issue-%d", state.Issue.Number)

	prNumber, prURL, err := s.client.CreatePR(ctx, owner, repo, title, body, branch, state.Repo.DefaultBranch)
	if err != nil {
		return StageResult{}, fmt.Errorf("create PR: %w", err)
	}

	state.PRNumber = prNumber
	state.PRURL = prURL

	return StageResult{
		Status: StatusPassed,
		Output: fmt.Sprintf("PR #%d created: %s", prNumber, prURL),
	}, nil
}
```

**Step 2: Implement review stage**

Create `internal/pipeline/review.go`:
```go
package pipeline

import (
	"context"
	"fmt"
	"strings"

	gh "github.com/mandadapu/neuralforge/internal/github"
	"github.com/mandadapu/neuralforge/internal/llm"
)

type ReviewStage struct {
	llm    llm.LLM
	client *gh.Client
}

func NewReviewStage(l llm.LLM, client *gh.Client) *ReviewStage {
	return &ReviewStage{llm: l, client: client}
}

func (s *ReviewStage) Name() string { return "review" }

func (s *ReviewStage) Run(ctx context.Context, state *PipelineState) (StageResult, error) {
	if state.PRNumber == 0 {
		return StageResult{Status: StatusSkipped, Output: "no PR to review"}, nil
	}

	var changesDesc strings.Builder
	for _, c := range state.Changes {
		fmt.Fprintf(&changesDesc, "- %s (%s)\n", c.Path, c.Action)
	}

	prompt := fmt.Sprintf(`Review the following changes for a PR:

## Issue: #%d %s
## Files Changed:
%s
## Plan:
%s
## Test Results:
%s

Provide a code review. If there are critical issues, say "REQUEST_CHANGES".
If the code looks good, say "APPROVE".`, state.Issue.Number, state.Issue.Title,
		changesDesc.String(), state.Plan,
		func() string {
			if state.TestResults != nil {
				return state.TestResults.Output
			}
			return "no tests run"
		}())

	resp, err := s.llm.Complete(ctx, llm.CompletionRequest{
		System:    "You are a senior code reviewer. Be thorough but constructive.",
		Messages:  []llm.Message{{Role: llm.RoleUser, Content: prompt}},
		MaxTokens: 2048,
	})
	if err != nil {
		return StageResult{}, fmt.Errorf("review LLM call: %w", err)
	}

	state.ReviewNotes = resp.Content
	state.Cost += resp.Cost

	event := "APPROVE"
	if strings.Contains(resp.Content, "REQUEST_CHANGES") {
		event = "REQUEST_CHANGES"
	}

	parts := strings.SplitN(state.Repo.FullName, "/", 2)
	if len(parts) == 2 {
		_ = s.client.CreateReview(ctx, parts[0], parts[1], state.PRNumber, resp.Content, event)
	}

	return StageResult{
		Status: StatusPassed,
		Output: fmt.Sprintf("Review: %s (%d tokens)", event, resp.OutputTokens),
	}, nil
}
```

**Step 3: Implement merge stage**

Create `internal/pipeline/merge.go`:
```go
package pipeline

import (
	"context"
	"fmt"
	"strings"

	gh "github.com/mandadapu/neuralforge/internal/github"
)

type MergeStage struct {
	client    *gh.Client
	autoMerge bool
}

func NewMergeStage(client *gh.Client, autoMerge bool) *MergeStage {
	return &MergeStage{client: client, autoMerge: autoMerge}
}

func (s *MergeStage) Name() string { return "merge" }

func (s *MergeStage) Run(ctx context.Context, state *PipelineState) (StageResult, error) {
	if !s.autoMerge {
		return StageResult{Status: StatusSkipped, Output: "auto-merge disabled"}, nil
	}
	if state.PRNumber == 0 {
		return StageResult{Status: StatusSkipped, Output: "no PR to merge"}, nil
	}
	if strings.Contains(state.ReviewNotes, "REQUEST_CHANGES") {
		return StageResult{
			Status: StatusFailed,
			Output: "Review requested changes — skipping merge",
		}, nil
	}

	parts := strings.SplitN(state.Repo.FullName, "/", 2)
	if len(parts) != 2 {
		return StageResult{}, fmt.Errorf("invalid repo: %s", state.Repo.FullName)
	}

	msg := fmt.Sprintf("Auto-merged by NeuralForge for issue #%d", state.Issue.Number)
	if err := s.client.MergePR(ctx, parts[0], parts[1], state.PRNumber, msg); err != nil {
		return StageResult{
			Status: StatusFailed,
			Output: fmt.Sprintf("Merge failed: %v", err),
		}, nil
	}

	state.Merged = true
	return StageResult{
		Status: StatusPassed,
		Output: fmt.Sprintf("PR #%d merged", state.PRNumber),
	}, nil
}
```

**Step 4: Implement deploy stage**

Create `internal/pipeline/deploy.go`:
```go
package pipeline

import "context"

type DeployStage struct {
	enabled bool
}

func NewDeployStage(enabled bool) *DeployStage {
	return &DeployStage{enabled: enabled}
}

func (s *DeployStage) Name() string { return "deploy" }

func (s *DeployStage) Run(_ context.Context, state *PipelineState) (StageResult, error) {
	if !s.enabled {
		return StageResult{Status: StatusSkipped, Output: "CI/deploy disabled"}, nil
	}
	if !state.Merged {
		return StageResult{Status: StatusSkipped, Output: "PR not merged"}, nil
	}
	// v1: Deploy is triggered by GitHub CI on merge — nothing to do here
	return StageResult{
		Status: StatusPassed,
		Output: "Deploy triggered via CI on merge",
	}, nil
}
```

**Step 5: Build**

```bash
go build ./...
```

**Step 6: Commit**

```bash
git add -A && git commit -m "feat: add pipeline stages — PR, review, merge, deploy"
```

---

## Phase 5: Worker Pool & Docker Executor (Tasks 12-13)

### Task 12: Worker Pool

**Files:**
- Create: `internal/worker/pool.go`
- Create: `internal/worker/worker.go`
- Create: `internal/worker/pool_test.go`

**Step 1: Write pool test**

Create `internal/worker/pool_test.go`:
```go
package worker

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mandadapu/neuralforge/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockStore struct {
	store.Store
	jobs    []store.Job
	updated int32
}

func (m *mockStore) ListPendingJobs(limit int) ([]store.Job, error) {
	if len(m.jobs) == 0 {
		return nil, nil
	}
	j := m.jobs[0]
	m.jobs = m.jobs[1:]
	return []store.Job{j}, nil
}

func (m *mockStore) UpdateJobStatus(id string, status store.JobStatus, stage string) error {
	atomic.AddInt32(&m.updated, 1)
	return nil
}

func (m *mockStore) CompleteJob(id string, status store.JobStatus) error {
	return nil
}

func (m *mockStore) UpdateJobError(id string, errMsg string) error {
	return nil
}

func TestPoolStartsAndStops(t *testing.T) {
	ms := &mockStore{}
	p := NewPool(3, ms, nil)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := p.Start(ctx)
	require.NoError(t, err)
}

func TestPoolProcessesJob(t *testing.T) {
	ms := &mockStore{
		jobs: []store.Job{
			{ID: "j1", RepoFullName: "o/r", IssueNumber: 1, Status: store.JobQueued},
		},
	}

	var processed int32
	handler := func(ctx context.Context, job store.Job) error {
		atomic.AddInt32(&processed, 1)
		return nil
	}

	p := NewPool(1, ms, handler)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	p.Start(ctx)
	time.Sleep(500 * time.Millisecond)

	assert.Equal(t, int32(1), atomic.LoadInt32(&processed))
}
```

**Step 2: Implement pool**

Create `internal/worker/pool.go`:
```go
package worker

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/mandadapu/neuralforge/internal/store"
)

type JobHandler func(ctx context.Context, job store.Job) error

type Pool struct {
	size    int
	store   store.Store
	handler JobHandler
	jobs    chan store.Job
}

func NewPool(size int, s store.Store, handler JobHandler) *Pool {
	return &Pool{
		size:    size,
		store:   s,
		handler: handler,
		jobs:    make(chan store.Job, size*2),
	}
}

func (p *Pool) Start(ctx context.Context) error {
	var wg sync.WaitGroup

	// Start workers
	for i := 0; i < p.size; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			w := &Worker{id: id, handler: p.handler, store: p.store}
			w.Run(ctx, p.jobs)
		}(i)
	}

	// Start poller
	go p.poll(ctx)

	// Wait for context cancellation
	go func() {
		<-ctx.Done()
		close(p.jobs)
		wg.Wait()
	}()

	return nil
}

func (p *Pool) poll(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			jobs, err := p.store.ListPendingJobs(p.size)
			if err != nil {
				slog.Error("poll error", "error", err)
				continue
			}
			for _, job := range jobs {
				if err := p.store.UpdateJobStatus(job.ID, store.JobRunning, ""); err != nil {
					slog.Error("update job status", "error", err)
					continue
				}
				select {
				case p.jobs <- job:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}
```

**Step 3: Implement worker**

Create `internal/worker/worker.go`:
```go
package worker

import (
	"context"
	"log/slog"

	"github.com/mandadapu/neuralforge/internal/store"
)

type Worker struct {
	id      int
	handler JobHandler
	store   store.Store
}

func (w *Worker) Run(ctx context.Context, jobs <-chan store.Job) {
	for job := range jobs {
		select {
		case <-ctx.Done():
			return
		default:
		}

		slog.Info("worker processing job", "worker", w.id, "job", job.ID)

		if err := w.handler(ctx, job); err != nil {
			slog.Error("job failed", "worker", w.id, "job", job.ID, "error", err)
			_ = w.store.UpdateJobError(job.ID, err.Error())
			continue
		}

		_ = w.store.CompleteJob(job.ID, store.JobCompleted)
		slog.Info("job completed", "worker", w.id, "job", job.ID)
	}
}
```

**Step 4: Run tests**

```bash
go test ./internal/worker/ -v
```

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: add worker pool with configurable concurrency"
```

---

### Task 13: Docker Executor

**Files:**
- Create: `internal/executor/docker.go`
- Create: `internal/executor/docker_test.go`

**Step 1: Write test**

Create `internal/executor/docker_test.go`:
```go
package executor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDockerJobArgs(t *testing.T) {
	d := NewDocker("ghcr.io/test:latest")

	job := ExecutorJob{
		ID:       "test-1",
		RepoPath: "/tmp/repo",
		Branch:   "fix-1",
		Prompt:   "implement changes",
		Timeout:  10 * time.Minute,
	}

	args := d.buildArgs(job)
	assert.Contains(t, args, "--rm")
	assert.Contains(t, args, "ghcr.io/test:latest")
}
```

**Step 2: Implement Docker executor**

Create `internal/executor/docker.go`:
```go
package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type DockerExecutor struct {
	image string
}

func NewDocker(image string) *DockerExecutor {
	return &DockerExecutor{image: image}
}

func (d *DockerExecutor) Name() string { return "docker" }

func (d *DockerExecutor) buildArgs(job ExecutorJob) []string {
	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/workspace", job.RepoPath),
		"-w", "/workspace",
		"-e", fmt.Sprintf("BRANCH=%s", job.Branch),
		"-e", fmt.Sprintf("JOB_ID=%s", job.ID),
	}
	for k, v := range job.EnvVars {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, d.image)
	return args
}

func (d *DockerExecutor) Run(ctx context.Context, job ExecutorJob) (ExecutorResult, error) {
	ctx, cancel := context.WithTimeout(ctx, job.Timeout)
	defer cancel()

	args := d.buildArgs(job)
	// Pass prompt via stdin
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = strings.NewReader(job.Prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	timedOut := ctx.Err() != nil

	return ExecutorResult{
		Success:  err == nil,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		TimedOut: timedOut,
	}, nil
}

func (d *DockerExecutor) Cleanup(ctx context.Context, jobID string) error {
	// Kill any running container for this job
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", fmt.Sprintf("nf-%s", jobID))
	return cmd.Run()
}
```

**Step 3: Run tests**

```bash
go test ./internal/executor/ -v
```

**Step 4: Commit**

```bash
git add -A && git commit -m "feat: add Docker executor"
```

---

## Phase 6: Webhook Server & CLI (Tasks 14-15)

### Task 14: Webhook Server

**Files:**
- Create: `internal/app/app.go`
- Create: `internal/app/webhook.go`
- Create: `internal/app/webhook_test.go`

**Step 1: Write webhook test**

Create `internal/app/webhook_test.go`:
```go
package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWebhookSignatureValidation(t *testing.T) {
	secret := "test-secret"
	body := `{"action":"labeled"}`

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	handler := NewWebhookHandler(secret, func(eventType string, payload []byte) {})

	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader(body))
	req.Header.Set("X-Hub-Signature-256", sig)
	req.Header.Set("X-GitHub-Event", "issues")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestWebhookRejectsInvalidSignature(t *testing.T) {
	handler := NewWebhookHandler("secret", func(eventType string, payload []byte) {})

	req := httptest.NewRequest("POST", "/webhooks/github", strings.NewReader("{}"))
	req.Header.Set("X-Hub-Signature-256", "sha256=invalid")
	req.Header.Set("X-GitHub-Event", "issues")

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
```

**Step 2: Implement webhook handler**

Create `internal/app/webhook.go`:
```go
package app

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"strings"
)

type EventCallback func(eventType string, payload []byte)

type WebhookHandler struct {
	secret   string
	callback EventCallback
}

func NewWebhookHandler(secret string, callback EventCallback) *WebhookHandler {
	return &WebhookHandler{secret: secret, callback: callback}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "read error", http.StatusBadRequest)
		return
	}

	sig := r.Header.Get("X-Hub-Signature-256")
	if !h.verifySignature(body, sig) {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	eventType := r.Header.Get("X-GitHub-Event")
	go h.callback(eventType, body)

	w.WriteHeader(http.StatusOK)
	w.Write([]byte(`{"ok":true}`))
}

func (h *WebhookHandler) verifySignature(payload []byte, signature string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	sig, err := hex.DecodeString(strings.TrimPrefix(signature, "sha256="))
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(h.secret))
	mac.Write(payload)
	return hmac.Equal(sig, mac.Sum(nil))
}
```

**Step 3: Implement app lifecycle**

Create `internal/app/app.go`:
```go
package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/mandadapu/neuralforge/internal/config"
	gh "github.com/mandadapu/neuralforge/internal/github"
	"github.com/mandadapu/neuralforge/internal/store"
	"github.com/mandadapu/neuralforge/internal/worker"
)

type App struct {
	cfg    config.Config
	store  store.Store
	pool   *worker.Pool
	server *http.Server
}

func New(cfg config.Config) (*App, error) {
	s, err := store.NewSQLiteStore(cfg.Store.DSN)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := s.Migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	app := &App{cfg: cfg, store: s}
	return app, nil
}

func (a *App) Start(ctx context.Context) error {
	// Build job handler (wires up the full pipeline)
	handler := a.buildJobHandler()

	// Start worker pool
	a.pool = worker.NewPool(a.cfg.Workers, a.store, handler)
	if err := a.pool.Start(ctx); err != nil {
		return fmt.Errorf("start pool: %w", err)
	}

	// Setup HTTP
	mux := http.NewServeMux()
	mux.Handle("/webhooks/github", NewWebhookHandler(a.cfg.GitHub.WebhookSecret, a.handleEvent))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok"}`))
	})

	a.server = &http.Server{
		Addr:    fmt.Sprintf("%s:%d", a.cfg.Server.Host, a.cfg.Server.Port),
		Handler: mux,
	}

	slog.Info("starting server", "addr", a.server.Addr, "workers", a.cfg.Workers)
	return a.server.ListenAndServe()
}

func (a *App) Shutdown(ctx context.Context) error {
	if a.server != nil {
		a.server.Shutdown(ctx)
	}
	if a.store != nil {
		a.store.Close()
	}
	return nil
}

func (a *App) handleEvent(eventType string, payload []byte) {
	evt, err := gh.ParseWebhookEvent(eventType, payload)
	if err != nil {
		slog.Error("parse event", "error", err)
		return
	}
	if evt == nil {
		return
	}

	switch e := evt.(type) {
	case *gh.IssueLabeledEvent:
		if e.Label != "neuralforge" {
			return
		}
		job := store.Job{
			ID:           fmt.Sprintf("nf-%d-%d", time.Now().UnixMilli(), e.Issue.Number),
			RepoFullName: e.Repo.FullName,
			IssueNumber:  e.Issue.Number,
			IssueTitle:   e.Issue.Title,
			Status:       store.JobQueued,
		}
		if err := a.store.CreateJob(job); err != nil {
			slog.Error("create job", "error", err)
		}
	}
}

func (a *App) buildJobHandler() worker.JobHandler {
	return func(ctx context.Context, job store.Job) error {
		slog.Info("processing job", "id", job.ID, "repo", job.RepoFullName, "issue", job.IssueNumber)
		// Full pipeline wiring happens here — Task 16
		return nil
	}
}
```

**Step 4: Run tests**

```bash
go test ./internal/app/ -v
```

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: add webhook server with HMAC signature verification"
```

---

### Task 15: CLI with Cobra

**Files:**
- Modify: `cmd/neuralforge/main.go`

**Step 1: Install cobra**

```bash
go get github.com/spf13/cobra
```

**Step 2: Rewrite main.go with cobra commands**

Replace `cmd/neuralforge/main.go`:
```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/mandadapu/neuralforge/internal/app"
	"github.com/mandadapu/neuralforge/internal/config"
	"github.com/spf13/cobra"
)

var version = "0.1.0-dev"

func main() {
	root := &cobra.Command{
		Use:   "neuralforge",
		Short: "Autonomous software factory",
	}

	root.AddCommand(serveCmd())
	root.AddCommand(versionCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func serveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Start the webhook server and worker pool",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := config.LoadFromEnv()

			a, err := app.New(cfg)
			if err != nil {
				return err
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			// Graceful shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				slog.Info("shutting down")
				cancel()
				a.Shutdown(context.Background())
			}()

			return a.Start(ctx)
		},
	}
}

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("neuralforge %s\n", version)
		},
	}
}
```

**Step 3: Build and verify**

```bash
go mod tidy && go build -o bin/neuralforge ./cmd/neuralforge
bin/neuralforge version
```
Expected: `neuralforge 0.1.0-dev`

**Step 4: Commit**

```bash
git add -A && git commit -m "feat: add CLI with serve and version commands"
```

---

## Phase 7: Integration & Dockerfile (Task 16)

### Task 16: Wire Pipeline & Build Dockerfile

**Files:**
- Modify: `internal/app/app.go` (wire up full pipeline in `buildJobHandler`)
- Create: `Dockerfile`
- Create: `.neuralforge.yml.example`

**Step 1: Wire full pipeline in buildJobHandler**

Update `buildJobHandler` in `internal/app/app.go` to:
```go
func (a *App) buildJobHandler() worker.JobHandler {
	// Create LLM backend
	var llmBackend llm.LLM
	switch a.cfg.LLM.DefaultProvider {
	case "openai":
		llmBackend = llm.NewOpenAI(a.cfg.LLM.OpenAI.APIKey, a.cfg.LLM.OpenAI.Model)
	default:
		llmBackend = llm.NewClaude(a.cfg.LLM.Claude.APIKey, a.cfg.LLM.Claude.Model)
	}

	// Create executor
	exec := executor.NewDocker(a.cfg.Executor.Docker.Image)

	return func(ctx context.Context, job store.Job) error {
		// Clone repo
		dir := filepath.Join(os.TempDir(), "neuralforge", job.ID)
		g, err := gitpkg.Clone(job.RepoFullName, dir, "")  // token resolved at runtime
		if err != nil {
			return fmt.Errorf("clone: %w", err)
		}
		defer os.RemoveAll(dir)

		// Build pipeline state
		state := &pipeline.PipelineState{
			JobID: job.ID,
			Issue: pipeline.GitHubIssue{
				Number: job.IssueNumber,
				Title:  job.IssueTitle,
			},
			Repo: pipeline.RepoContext{
				FullName:      job.RepoFullName,
				DefaultBranch: "main",
				LocalPath:     g.Dir(),
			},
		}

		// Build stages
		stages := []pipeline.Stage{
			pipeline.NewArchitectStage(llmBackend),
			pipeline.NewSecurityStage(llmBackend),
			pipeline.NewExecuteStage(exec, a.cfg.Executor.Docker.Timeout),
			pipeline.NewVerifyStage("make test"),
			pipeline.NewComplianceStage(2000, 50),
			// PR, review, merge, deploy need GitHub client — wired when GitHub App auth is added
		}

		engine := pipeline.NewEngine(stages, &pipeline.EngineConfig{BudgetUSD: 5.0})

		// Update store on stage progress
		engine.OnStageComplete(func(s *pipeline.PipelineState, stage string, status pipeline.StageStatus) {
			a.store.UpdateJobStatus(s.JobID, store.JobRunning, stage)
		})

		return engine.Run(ctx, state)
	}
}
```

**Step 2: Create Dockerfile**

```dockerfile
FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o /neuralforge ./cmd/neuralforge

FROM alpine:3.19
RUN apk add --no-cache git docker-cli ca-certificates
COPY --from=builder /neuralforge /usr/local/bin/neuralforge
ENTRYPOINT ["neuralforge"]
CMD ["serve"]
```

**Step 3: Create .neuralforge.yml.example**

```yaml
neuralforge:
  trigger:
    label: "neuralforge"

  llm:
    provider: claude
    model: claude-sonnet-4-5-20250514

  executor:
    type: docker

  pipeline:
    architecture_review: true
    security_review: true
    verification:
      command: "make test"
    compliance:
      max_diff_lines: 2000
      blocked_licenses: [AGPL-3.0]
    code_review: true
    auto_merge: false
    ci_deploy: false

  limits:
    max_files_changed: 50
    timeout_minutes: 30
    budget_usd: 5.0
```

**Step 4: Build everything**

```bash
go mod tidy && go build ./... && go test ./... -count=1
```

**Step 5: Commit**

```bash
git add -A && git commit -m "feat: wire full pipeline, add Dockerfile and config example"
```

---

## Summary

| Phase | Tasks | What it builds |
|-------|-------|---------------|
| 1 | 1-3 | Scaffold, interfaces, config |
| 2 | 4-5 | SQLite store, git operations |
| 3 | 6-8 | GitHub client, Claude + OpenAI backends |
| 4 | 9-11 | Pipeline engine + all 10 stages |
| 5 | 12-13 | Worker pool, Docker executor |
| 6 | 14-15 | Webhook server, CLI |
| 7 | 16 | Integration wiring, Dockerfile |

**Total: 16 tasks, ~80 steps**

Each task is independently testable. Phases can be parallelized (e.g., Phase 2 and Phase 3 are independent).
