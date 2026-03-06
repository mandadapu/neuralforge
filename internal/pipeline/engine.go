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

		if e.config.BudgetUSD > 0 && state.Cost > e.config.BudgetUSD {
			return fmt.Errorf("budget exceeded: $%.2f > $%.2f limit", state.Cost, e.config.BudgetUSD)
		}

		if result.Status == StatusFailed {
			return fmt.Errorf("stage %s failed: %s", stage.Name(), result.Output)
		}
	}

	return nil
}
