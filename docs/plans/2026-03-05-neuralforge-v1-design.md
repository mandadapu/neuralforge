# NeuralForge v1 — Design Document

## Vision

NeuralForge is a standalone **autonomous software factory**. It installs as a GitHub App, watches for labeled issues, and autonomously produces reviewed, tested, compliant pull requests — then merges and deploys them.

It is LLM-agnostic, executor-agnostic, and designed to be reusable across any codebase.

## Pipeline

```
Issue
  → Codebase Context (build/refresh CLAUDE.md)
  → Architecture Review (plan the implementation)
  → Security Review (check plan for risks)
  → Implementation (execute via pluggable executor)
  → Verification (tests, lint, type-check)
  → Compliance (license, policy, diff-size gates)
  → Pull Request (open PR with full context)
  → Code Review (LLM reviews its own changes)
  → Merge (auto-merge if all checks pass)
  → CI/Deploy (trigger if enabled)
```

Each stage is a Go interface. Stages can be enabled/disabled via `.neuralforge.yml`.

## Architecture

```
┌─────────────────────────────────────────────────────┐
│               GitHub App (webhook receiver)          │
│               POST /webhooks/github                  │
└──────────────────┬──────────────────────────────────┘
                   │ issues.labeled
                   ▼
┌─────────────────────────────────────────────────────┐
│              Job Queue (buffered Go channel)          │
│              Persisted to SQLite/Postgres             │
└──────────────────┬──────────────────────────────────┘
                   │
        ┌──────────┼──────────┬──────────┬──────────┐
        ▼          ▼          ▼          ▼          ▼
    Worker 1   Worker 2   Worker 3   Worker 4   Worker 5
    (pipeline) (pipeline) (pipeline) (pipeline) (pipeline)
        │          │          │          │          │
        ▼          ▼          ▼          ▼          ▼
    ┌────────┐ ┌────────┐
    │Executor│ │  LLM   │  (pluggable backends)
    │Interface│ │Interface│
    ├────────┤ ├────────┤
    │ Docker │ │ Claude │
    │ VM/SSH │ │ OpenAI │
    │ K8s    │ │ Gemini │
    │ Local  │ │ Ollama │
    └────────┘ └────────┘
```

## Codebase Understanding

Before processing any issue, NeuralForge builds deep codebase context.

### First-Time Analysis

When a repo has no `CLAUDE.md`, the context analyzer performs:

1. Language and framework detection
2. Project structure mapping (key dirs, entry points)
3. Architecture pattern inference (monolith, microservices, monorepo)
4. Dependency graph analysis (imports, packages, modules)
5. Test infrastructure detection (framework, coverage commands)
6. Build system mapping (Makefile, package.json, Dockerfile, CI config)
7. Code convention extraction (naming, formatting, patterns)
8. Key abstraction identification (interfaces, base classes, shared utils)
9. Database schema and migration discovery
10. API surface mapping (routes, endpoints, contracts)

Result is committed as `CLAUDE.md` to the repo.

### Context Refresh

Existing `CLAUDE.md` is refreshed when:
- Older than configurable interval (default 7 days)
- More than 50 files changed since last update

### Context Injection

Every LLM call in every pipeline stage receives the CLAUDE.md as system context. Workers never operate without full codebase understanding.

## Worker Pool

5 parallel workers (configurable via `NEURALFORGE_WORKERS` env var or config). Each worker:

1. Pulls a job from the queue
2. Clones the repo (or uses cached bare repo with worktrees)
3. Ensures CLAUDE.md context exists and is fresh
4. Runs the full pipeline
5. Reports result back to GitHub (PR or comment on failure)

Workers are independent — no shared state beyond the job queue and database.

## Project Structure

```
neuralforge/
├── cmd/
│   └── neuralforge/
│       └── main.go                 # Entry point, CLI
├── internal/
│   ├── app/
│   │   ├── app.go                  # Application lifecycle
│   │   └── webhook.go              # GitHub webhook handler
│   ├── config/
│   │   ├── config.go               # Global config (env, flags)
│   │   └── repoconfig.go           # Per-repo .neuralforge.yml
│   ├── context/
│   │   ├── analyzer.go             # Deep codebase analysis
│   │   ├── manager.go              # CLAUDE.md lifecycle (create/refresh)
│   │   └── memory.go               # RepoMemory struct and rendering
│   ├── pipeline/
│   │   ├── engine.go               # Sequential stage runner
│   │   ├── stage.go                # Stage interface definition
│   │   ├── state.go                # PipelineState struct
│   │   ├── architect.go            # Architecture review stage
│   │   ├── security.go             # Security review stage
│   │   ├── execute.go              # Implementation dispatch stage
│   │   ├── verify.go               # Test/lint/type-check stage
│   │   ├── compliance.go           # Policy/license/diff-size gates
│   │   ├── pr.go                   # PR creation stage
│   │   ├── review.go               # Code review stage
│   │   ├── merge.go                # Auto-merge stage
│   │   └── deploy.go               # CI trigger stage
│   ├── executor/
│   │   ├── executor.go             # Executor interface
│   │   ├── docker.go               # Docker container executor
│   │   ├── vm.go                   # SSH/VM executor
│   │   └── local.go                # Local process executor
│   ├── llm/
│   │   ├── llm.go                  # LLM interface
│   │   ├── claude.go               # Anthropic Claude backend
│   │   ├── openai.go               # OpenAI GPT backend
│   │   └── gemini.go               # Google Gemini backend
│   ├── git/
│   │   ├── git.go                  # Git operations (clone, branch, commit, push)
│   │   └── worktree.go             # Bare repo + worktree caching
│   ├── github/
│   │   ├── client.go               # GitHub API (PRs, reviews, merges, comments)
│   │   └── events.go               # Webhook event parsing
│   ├── worker/
│   │   ├── pool.go                 # Worker pool with buffered channel
│   │   └── worker.go               # Single worker loop
│   └── store/
│       ├── store.go                # Store interface
│       ├── sqlite.go               # SQLite implementation
│       └── postgres.go             # PostgreSQL implementation
├── .neuralforge.yml.example        # Repo-level config template
├── Dockerfile
├── go.mod
├── go.sum
├── Makefile
├── CLAUDE.md
└── README.md
```

## Key Interfaces

### LLM

```go
type LLM interface {
    Complete(ctx context.Context, req CompletionRequest) (CompletionResponse, error)
    StreamComplete(ctx context.Context, req CompletionRequest) (<-chan StreamChunk, error)
    Name() string
}

type CompletionRequest struct {
    System   string
    Messages []Message
    Model    string
    MaxTokens int
    Temperature float64
}
```

### Executor

```go
type Executor interface {
    Run(ctx context.Context, job ExecutorJob) (ExecutorResult, error)
    Cleanup(ctx context.Context, jobID string) error
    Name() string
}

type ExecutorJob struct {
    ID         string
    RepoPath   string
    Branch     string
    Prompt     string            // implementation instructions
    Context    string            // CLAUDE.md content
    Timeout    time.Duration
    EnvVars    map[string]string
}

type ExecutorResult struct {
    Success    bool
    Stdout     string
    Stderr     string
    FilesChanged []string
    TimedOut   bool
}
```

### Pipeline Stage

```go
type Stage interface {
    Name() string
    Run(ctx context.Context, state *PipelineState) (StageResult, error)
}

type StageResult struct {
    Status  StageStatus  // passed, failed, skipped
    Output  string       // stage output for logging
    Details map[string]any
}
```

## Pipeline State

```go
type PipelineState struct {
    // Input
    JobID       string
    Issue       GitHubIssue
    Repo        RepoContext

    // Context
    Memory      *RepoMemory     // parsed CLAUDE.md

    // Stage outputs
    Plan        string          // architect output
    SecurityNotes string        // security review output
    Changes     []FileChange    // executor output
    TestResults *TestReport     // verification output
    Compliance  *ComplianceReport
    PRURL       string          // created PR URL
    PRNumber    int
    ReviewNotes string          // code review output
    Merged      bool
    DeployURL   string          // CI trigger result

    // Metadata
    StartedAt   time.Time
    Cost        float64         // accumulated LLM cost
    Stages      []StageLog      // audit trail
}
```

## Persistence

SQLite by default (WAL mode for concurrent worker writes), Postgres optional.

```sql
CREATE TABLE jobs (
    id TEXT PRIMARY KEY,
    repo_full_name TEXT NOT NULL,
    issue_number INTEGER NOT NULL,
    issue_title TEXT,
    status TEXT DEFAULT 'queued',
    current_stage TEXT,
    pipeline_state TEXT,          -- JSON blob
    error TEXT,
    cost_usd REAL DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    UNIQUE(repo_full_name, issue_number)
);

CREATE TABLE repo_contexts (
    repo_full_name TEXT PRIMARY KEY,
    claude_md_hash TEXT,
    last_analyzed_at TIMESTAMP,
    file_count INTEGER,
    languages TEXT,               -- JSON array
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
```

## Configuration

### Global (env vars or `config.yml`)

```yaml
server:
  port: 8080
  host: 0.0.0.0

workers: 5

github:
  app_id: 12345
  private_key_path: /etc/neuralforge/github-app.pem
  webhook_secret: "whsec_..."

llm:
  default_provider: claude
  claude:
    api_key: "sk-ant-..."
  openai:
    api_key: "sk-..."
  gemini:
    api_key: "AI..."

executor:
  default_type: docker
  docker:
    image: "ghcr.io/neuralforge/executor:latest"
    timeout: 30m

store:
  driver: sqlite              # sqlite | postgres
  dsn: "neuralforge.db"

context:
  auto_generate: true
  refresh_days: 7
  analysis_depth: thorough    # quick | medium | thorough
  commit_to_repo: true
```

### Per-repo (`.neuralforge.yml`)

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

## GitHub App Events

| Event | Trigger | Action |
|-------|---------|--------|
| `issues.labeled` | Label matches config | Enqueue pipeline job |
| `issue_comment.created` | `/retry` command | Re-enqueue failed job |
| `issue_comment.created` | `/cancel` command | Cancel in-progress job |
| `issue_comment.created` | `/status` command | Post current pipeline status |
| `pull_request.closed` | PR merged | Update job status to done |
| `pull_request.closed` | PR rejected | Update job status, comment on issue |
| `check_suite.completed` | All checks pass | Trigger deploy stage if enabled |

## CLI

```bash
# Start the server
neuralforge serve

# Start with custom config
neuralforge serve --config /etc/neuralforge/config.yml

# Process a single issue (for testing)
neuralforge run --repo owner/repo --issue 42

# Analyze a repo and generate CLAUDE.md
neuralforge analyze --repo owner/repo

# Check config
neuralforge config validate

# Version
neuralforge version
```

## Error Handling

- Each stage failure is reported as a comment on the GitHub issue
- Failed jobs can be retried via `/retry` comment
- Workers auto-recover from panics (supervisor restarts)
- LLM API errors retry with exponential backoff (3 attempts)
- Executor timeouts kill the container/process and fail the job
- Budget exceeded stops the pipeline and comments on the issue

## Cost Tracking

Every LLM call tracks token usage and cost. Pipeline stops if:
- Per-job budget exceeded (configurable, default $5)
- Total cost reported in PR description and issue comment

## Security

- GitHub webhook signature verification (HMAC-SHA256)
- Private key stored securely (file path, not env var)
- Executor containers run with no network access except GitHub/LLM APIs
- No secrets in pipeline state (tokens resolved at runtime)
- Git operations use short-lived installation tokens

## v1 Scope

### Included
- Go binary with `serve`, `run`, `analyze` commands
- GitHub App webhook handler
- 5 parallel workers (configurable)
- Full 10-stage pipeline
- CLAUDE.md auto-generation and refresh
- Claude + OpenAI LLM backends
- Docker executor backend
- SQLite persistence
- `.neuralforge.yml` per-repo config
- Cost tracking and budget limits

### Excluded (future)
- Web dashboard
- REST API for non-GitHub integrations
- GitLab / Jira / Bitbucket integrations
- Kubernetes executor
- Postgres persistence
- Multi-repo orchestration (epics)
- Ollama/local LLM support
