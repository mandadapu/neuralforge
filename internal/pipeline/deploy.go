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
		return StageResult{
			Status: StatusSkipped,
			Output: "Deploy stage disabled",
		}, nil
	}

	if !state.Merged {
		return StageResult{
			Status: StatusSkipped,
			Output: "PR not merged — skipping deploy",
		}, nil
	}

	return StageResult{
		Status: StatusPassed,
		Output: "Deploy triggered via CI on merge",
	}, nil
}
