# Claude Code K8s Executor — Design Document

## Goal

Add a `KubernetesExecutor` that runs Claude Code CLI (`claude -p --dangerously-skip-permissions`) inside K8s pods to autonomously implement code changes for NeuralForge pipeline jobs.

## Architecture

New executor implementing the existing `Executor` interface. For each job, creates a K8s Job with:

1. **Init container** (`alpine/git`) — clones the repo onto a shared `emptyDir` volume using a git token from K8s Secret
2. **Main container** (custom image with `claude` CLI) — runs Claude Code against the cloned repo, commits changes, pushes to `neuralforge/issue-{N}` branch

NeuralForge creates K8s Jobs via `client-go` (not kubectl), polls for completion, reads logs, then checks the remote branch for pushed commits.

## Pod Spec

```
K8s Job: neuralforge-{jobID}
├── Secret refs: neuralforge-llm-keys (ANTHROPIC_API_KEY or OAuth creds)
│                neuralforge-git-token (for clone + push)
├── Init container: git-clone
│   Image: alpine/git
│   Command: git clone --branch {base} {repo_url} /workspace
│   Volume: workspace (emptyDir)
├── Main container: claude-executor
│   Image: configurable (e.g., ghcr.io/neuralforge/claude-executor:latest)
│   Command: claude -p "{prompt}" --dangerously-skip-permissions
│   WorkDir: /workspace
│   Env: ANTHROPIC_API_KEY (from Secret), CLAUDE_CODE_MAX_TURNS (configurable)
│   Volume: workspace (emptyDir)
│   Post-run: git checkout -b neuralforge/issue-{N} && git push
└── Resource limits: configurable CPU/memory
```

## Auth

- **LLM keys**: K8s Secret (`neuralforge-llm-keys`) supports both `ANTHROPIC_API_KEY` and OAuth credentials. Mounted as env vars in the main container.
- **Git access**: K8s Secret (`neuralforge-git-token`) provides a token for clone and push. Used by both init container and main container.
- **In-cluster auth**: NeuralForge uses `client-go` in-cluster config when running in K8s. Supports kubeconfig fallback for local dev.

## Result Collection

1. Poll K8s Job status until complete/failed/timed out
2. Read pod logs (stdout/stderr) via K8s API
3. Check exit code for success/failure
4. Query remote repo for pushed branch to determine `FilesChanged`
5. Delete K8s Job on cleanup

## Error Handling

- **Timeout**: `activeDeadlineSeconds` on the K8s Job. NeuralForge marks as timed out.
- **Crash**: `backoffLimit: 0` — no K8s-level retries. NeuralForge handles retries at pipeline level.
- **No push on failure**: If Claude Code exits non-zero, no branch is pushed. NeuralForge detects missing remote branch and reports failure.
- **Cleanup**: Deletes K8s Job with `PropagationPolicy: Background`.
- **Namespace isolation**: All pods in dedicated namespace (default `neuralforge`).

## Configuration

```yaml
executor:
  default_type: kubernetes
  kubernetes:
    namespace: neuralforge
    image: "ghcr.io/neuralforge/claude-executor:latest"
    secret_name: "neuralforge-llm-keys"
    git_secret_name: "neuralforge-git-token"
    timeout: 30m
    resources:
      cpu: "2"
      memory: "4Gi"
    node_selector: {}
```

Env vars:
- `NEURALFORGE_EXECUTOR=kubernetes`
- `NEURALFORGE_K8S_NAMESPACE=neuralforge`
- `NEURALFORGE_K8S_IMAGE=ghcr.io/neuralforge/claude-executor:latest`
- `NEURALFORGE_K8S_SECRET=neuralforge-llm-keys`
- `NEURALFORGE_K8S_GIT_SECRET=neuralforge-git-token`
- `NEURALFORGE_K8S_CPU=2`
- `NEURALFORGE_K8S_MEMORY=4Gi`

## Files

| Action | File | What |
|--------|------|------|
| Create | `internal/executor/kubernetes.go` | `KubernetesExecutor` implementing `Executor` |
| Create | `internal/executor/kubernetes_test.go` | Unit tests with mock K8s client |
| Modify | `internal/config/config.go` | Add `KubernetesConfig` |
| Modify | `internal/config/config_test.go` | Test K8s config from env |
| Modify | `internal/app/app.go` | Wire K8s executor |
| Create | `deploy/claude-executor.Dockerfile` | Image with claude CLI + git |
| Create | `deploy/k8s-secrets.yaml.example` | Example Secret manifests |
