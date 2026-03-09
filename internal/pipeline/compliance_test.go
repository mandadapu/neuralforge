package pipeline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initComplianceRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()

	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	}
	for _, args := range cmds {
		require.NoError(t, exec.Command(args[0], args[1:]...).Run())
	}

	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# Test"), 0644))
	require.NoError(t, exec.Command("git", "-C", dir, "add", ".").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "-m", "init").Run())

	out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	require.NoError(t, err)

	// Use whatever branch git init created (main or master depending on git config)
	branch := strings.TrimSpace(string(out))

	// Create a feature branch with some changes
	require.NoError(t, exec.Command("git", "-C", dir, "checkout", "-b", "feature").Run())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.go"), []byte("package main\n\nfunc foo() {}\n"), 0644))
	require.NoError(t, exec.Command("git", "-C", dir, "add", ".").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "-m", "add new file").Run())

	return dir, branch
}

func TestComplianceStage_Name(t *testing.T) {
	s := NewComplianceStage(1000, 10)
	assert.Equal(t, "compliance", s.Name())
}

func TestComplianceStage_NoLocalPath(t *testing.T) {
	s := NewComplianceStage(1000, 10)
	state := &PipelineState{}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
	assert.Contains(t, result.Output, "no local path")
}

func TestComplianceStage_WithinLimits(t *testing.T) {
	dir, defaultBranch := initComplianceRepo(t)

	s := NewComplianceStage(1000, 50)
	state := &PipelineState{
		Repo: RepoContext{
			LocalPath:     dir,
			DefaultBranch: defaultBranch,
		},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	require.NotNil(t, state.Compliance)
	assert.True(t, state.Compliance.Passed)
	assert.Equal(t, 0, len(state.Compliance.Violations))
}

func TestComplianceStage_ExceedsDiffLimit(t *testing.T) {
	dir, defaultBranch := initComplianceRepo(t)

	// maxDiffLines=1 will be exceeded by the added file
	s := NewComplianceStage(1, 50)
	state := &PipelineState{
		Repo: RepoContext{
			LocalPath:     dir,
			DefaultBranch: defaultBranch,
		},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	require.NotNil(t, state.Compliance)
	assert.False(t, state.Compliance.Passed)
	assert.NotEmpty(t, state.Compliance.Violations)
}

func TestComplianceStage_ExceedsFileLimit(t *testing.T) {
	dir, defaultBranch := initComplianceRepo(t)

	// maxFilesChanged=0 means no limit; set maxFilesChanged=1 and add 2 files
	// Create a second file change
	require.NoError(t, os.WriteFile(filepath.Join(dir, "another.go"), []byte("package main\n"), 0644))
	require.NoError(t, exec.Command("git", "-C", dir, "add", ".").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "-m", "add another").Run())

	s := NewComplianceStage(10000, 1)
	state := &PipelineState{
		Repo: RepoContext{
			LocalPath:     dir,
			DefaultBranch: defaultBranch,
		},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	require.NotNil(t, state.Compliance)
	assert.False(t, state.Compliance.Passed)
}

func TestComplianceStage_ZeroLimitsNoViolation(t *testing.T) {
	dir, defaultBranch := initComplianceRepo(t)

	// Zero means no limit enforced
	s := NewComplianceStage(0, 0)
	state := &PipelineState{
		Repo: RepoContext{
			LocalPath:     dir,
			DefaultBranch: defaultBranch,
		},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
}
