package pipeline

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

type VerifyStage struct {
	command string
}

func NewVerifyStage(command string) *VerifyStage {
	if command == "" {
		command = "make test"
	}
	return &VerifyStage{command: command}
}

func (s *VerifyStage) Name() string { return "verify" }

func (s *VerifyStage) Run(_ context.Context, state *PipelineState) (StageResult, error) {
	if state.Repo.LocalPath == "" {
		return StageResult{
			Status: StatusSkipped,
			Output: "no local path available",
		}, nil
	}

	parts := strings.Fields(s.command)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Dir = state.Repo.LocalPath

	output, err := cmd.CombinedOutput()
	passed := err == nil

	state.TestResults = &TestReport{
		Passed:  passed,
		Output:  string(output),
		Command: s.command,
	}

	if !passed {
		return StageResult{
			Status: StatusFailed,
			Output: fmt.Sprintf("tests failed: %s", string(output)),
		}, nil
	}

	return StageResult{
		Status: StatusPassed,
		Output: "all tests passed",
	}, nil
}
