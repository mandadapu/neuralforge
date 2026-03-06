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
