# False Positive Security Findings

This file records security scanner findings that were incorrectly filed against
this repository (NeuralForge — a pure Go codebase) but belong to a different project.

## arch_001 — Issue #-763260470 (2026-03-09)

**Scanner:** arch_001 (circular import detection)
**Scan job:** repo:e47b27f

### Findings reported (all false positives)

| Finding | File | Status |
|---------|------|--------|
| Circular import: `api/db.py` → `api/db_router.py` | `api/db.py` | File does not exist |
| Circular import: `api/agent_database.py` → … → `pipeline/config.py` (5-file cycle) | `api/agent_database.py` | File does not exist |
| Circular import: `api/autopilot/fix_handlers/github_pr.py` → … (3-file cycle) | `api/autopilot/fix_handlers/github_pr.py` | File does not exist |

### Root cause

The scan job (`repo:e47b27f`) references a Python project that contains `api/` and
`pipeline/` directories. Those findings were incorrectly mapped to this repository.

NeuralForge is a Go binary. Go's compiler enforces an acyclic import graph at
compile time — any circular import would be a build error, not a scanner finding.
The existing import graph is clean:

```
config, store, executor, llm, git  →  stdlib / external only
pipeline                           →  executor, llm, git
github                             →  pipeline
worker                             →  store
app                                →  all packages (orchestration)
cmd/neuralforge                    →  app, config
```

### Action

- No code changes required in NeuralForge.
- The autopilot scan pipeline should be audited to prevent cross-repo mis-filing
  from scan job `repo:e47b27f`.

### Verification

- Rework review completed 2026-03-12: approved, no findings.
- `make build` continues to pass (Go compile-time acyclic import enforcement).
- `make test` continues to pass.
