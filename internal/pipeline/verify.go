package pipeline

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// allowedVerifyCommands is an allowlist of permitted build/test tool executables.
// This prevents repo-supplied verify commands from executing arbitrary shell code
// (e.g. "bash -c 'malicious'") on the NeuralForge server.
var allowedVerifyCommands = map[string]bool{
	"make":      true,
	"go":        true,
	"npm":       true,
	"yarn":      true,
	"pnpm":      true,
	"python":    true,
	"python3":   true,
	"pytest":    true,
	"mvn":       true,
	"gradle":    true,
	"gradlew":   true,
	"./gradlew": true,
	"cargo":     true,
	"bundle":    true,
	"rake":      true,
	"composer":  true,
	"mix":       true,
	"rebar3":    true,
}

type VerifyStage struct {
	command string
}

func NewVerifyStage(command string) *VerifyStage {
	command = strings.TrimSpace(command)
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
	if len(parts) == 0 {
		return StageResult{
			Status: StatusSkipped,
			Output: "no verify command configured",
		}, nil
	}
	if !allowedVerifyCommands[parts[0]] {
		return StageResult{
			Status: StatusFailed,
			Output: fmt.Sprintf("verify command not allowed: %s", parts[0]),
		}, nil
	}
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
