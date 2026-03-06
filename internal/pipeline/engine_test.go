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

type costMutatingStage struct {
	name    string
	addCost float64
}

func (m *costMutatingStage) Name() string { return m.name }
func (m *costMutatingStage) Run(_ context.Context, state *PipelineState) (StageResult, error) {
	state.Cost += m.addCost
	return StageResult{Status: StatusPassed, Output: "ok"}, nil
}

func TestEngineBudgetExceededMidStage(t *testing.T) {
	stages := []Stage{
		&costMutatingStage{name: "expensive", addCost: 3.0},
		&mockStage{name: "next", result: StageResult{Status: StatusPassed}},
	}
	e := NewEngine(stages, &EngineConfig{BudgetUSD: 2.0})

	state := &PipelineState{JobID: "test-budget-mid"}
	err := e.Run(context.Background(), state)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "budget exceeded")
	// The expensive stage ran and was logged, but "next" never ran
	assert.Len(t, state.Stages, 1)
	assert.Equal(t, "expensive", state.Stages[0].Name)
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
