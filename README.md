# NeuralForge

Autonomous software factory. Installs as a GitHub App, watches for labeled issues, and produces reviewed, tested, compliant pull requests — then merges and deploys them.

## How It Works

```
Issue (labeled "neuralforge")
  -> Codebase Context (build/refresh CLAUDE.md)
  -> Architecture Review (plan the implementation)
  -> Security Review (check plan for risks)
  -> Implementation (execute via pluggable executor)
  -> Verification (tests, lint, type-check)
  -> Compliance (license, policy, diff-size gates)
  -> Pull Request (open PR with full context)
  -> Code Review (LLM reviews its own changes)
  -> Merge (auto-merge if all checks pass)
  -> CI/Deploy (trigger if enabled)
```

## Features

- **LLM-agnostic** — Claude and OpenAI backends, extensible to others
- **Executor-agnostic** — Docker containers (VM, K8s planned)
- **5 parallel workers** — configurable via `NEURALFORGE_WORKERS`
- **Cost tracking** — per-job budget limits (default $5)
- **Per-repo config** — `.neuralforge.yml` for pipeline settings
- **CLAUDE.md auto-generation** — deep codebase analysis committed to repo
- **SQLite persistence** — WAL mode for concurrent writes

## Quick Start

```bash
# Build
make build

# Run
export ANTHROPIC_API_KEY="sk-ant-..."
export GITHUB_APP_ID="12345"
export GITHUB_WEBHOOK_SECRET="whsec_..."
export GITHUB_PRIVATE_KEY_PATH="/path/to/app.pem"

neuralforge serve
```

## CLI

```bash
neuralforge serve              # Start webhook server + worker pool
neuralforge serve --config config.yml  # Custom config
neuralforge version            # Print version
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `NEURALFORGE_PORT` | `8080` | Server port |
| `NEURALFORGE_WORKERS` | `5` | Parallel worker count |
| `ANTHROPIC_API_KEY` | — | Claude API key |
| `OPENAI_API_KEY` | — | OpenAI API key |
| `NEURALFORGE_LLM_PROVIDER` | `claude` | Default LLM (`claude` or `openai`) |
| `NEURALFORGE_EXECUTOR` | `docker` | Executor type |
| `NEURALFORGE_STORE_DSN` | `neuralforge.db` | SQLite database path |

### Per-Repo (`.neuralforge.yml`)

```yaml
neuralforge:
  trigger:
    label: "neuralforge"
  llm:
    provider: claude
    model: claude-sonnet-4-5-20250514
  pipeline:
    architecture_review: true
    security_review: true
    verification:
      command: "make test"
    compliance:
      max_diff_lines: 2000
    code_review: true
    auto_merge: false
  limits:
    max_files_changed: 50
    timeout_minutes: 30
    budget_usd: 5.0
```

## Project Structure

```
cmd/neuralforge/       CLI entry point (Cobra)
internal/
  app/                 Webhook server + app lifecycle
  config/              Global + per-repo configuration
  context/             CLAUDE.md analysis + management
  executor/            Docker executor (pluggable)
  git/                 Git operations wrapper
  github/              Webhook parser + GitHub API client
  llm/                 Claude + OpenAI backends
  pipeline/            Engine + 10 stages
  store/               SQLite persistence
  worker/              Worker pool
```

## GitHub App Events

| Event | Action |
|-------|--------|
| `issues.labeled` | Enqueue pipeline job |
| `issue_comment` `/retry` | Re-enqueue failed job |
| `issue_comment` `/cancel` | Cancel in-progress job |
| `issue_comment` `/status` | Post pipeline status |

## Development

```bash
make build     # Compile to bin/neuralforge
make test      # Run all tests with race detector
make lint      # Run golangci-lint
make clean     # Remove build artifacts
```

## License

MIT
