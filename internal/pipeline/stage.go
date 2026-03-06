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
