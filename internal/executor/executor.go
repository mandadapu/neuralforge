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
