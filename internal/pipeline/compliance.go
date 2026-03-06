package pipeline

import (
	"context"
	"fmt"

	"github.com/mandadapu/neuralforge/internal/git"
)

type ComplianceStage struct {
	maxDiffLines    int
	maxFilesChanged int
}

func NewComplianceStage(maxDiffLines, maxFilesChanged int) *ComplianceStage {
	return &ComplianceStage{
		maxDiffLines:    maxDiffLines,
		maxFilesChanged: maxFilesChanged,
	}
}

func (s *ComplianceStage) Name() string { return "compliance" }

func (s *ComplianceStage) Run(_ context.Context, state *PipelineState) (StageResult, error) {
	if state.Repo.LocalPath == "" {
		return StageResult{
			Status: StatusSkipped,
			Output: "no local path available",
		}, nil
	}

	g := git.New(state.Repo.LocalPath)

	files, err := g.FilesChanged(state.Repo.DefaultBranch)
	if err != nil {
		return StageResult{}, fmt.Errorf("compliance files changed: %w", err)
	}

	diffLines, err := g.DiffLines(state.Repo.DefaultBranch)
	if err != nil {
		return StageResult{}, fmt.Errorf("compliance diff lines: %w", err)
	}

	var violations []string
	if s.maxDiffLines > 0 && diffLines > s.maxDiffLines {
		violations = append(violations, fmt.Sprintf(
			"diff too large: %d lines exceeds limit of %d", diffLines, s.maxDiffLines,
		))
	}
	if s.maxFilesChanged > 0 && len(files) > s.maxFilesChanged {
		violations = append(violations, fmt.Sprintf(
			"too many files changed: %d exceeds limit of %d", len(files), s.maxFilesChanged,
		))
	}

	report := &ComplianceReport{
		Passed:       len(violations) == 0,
		DiffLines:    diffLines,
		FilesChanged: len(files),
		Violations:   violations,
	}
	state.Compliance = report

	if !report.Passed {
		return StageResult{
			Status: StatusFailed,
			Output: fmt.Sprintf("compliance violations: %v", violations),
		}, nil
	}

	return StageResult{
		Status: StatusPassed,
		Output: fmt.Sprintf("compliance passed: %d lines, %d files", diffLines, len(files)),
	}, nil
}
