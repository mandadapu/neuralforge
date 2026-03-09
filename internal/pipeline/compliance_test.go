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

// initComplianceRepo creates a minimal git repo on a feature branch with one
// added file, returning the repo dir and the default branch name.
func initComplianceRepo(t *testing.T) (dir, defaultBranch string) {
	t.Helper()

	_, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git binary not available")
	}

	dir = t.TempDir()
	cmds := [][]string{
		{"git", "init", dir},
		{"git", "-C", dir, "config", "user.email", "test@test.com"},
		{"git", "-C", dir, "config", "user.name", "Test"},
	}
	for _, c := range cmds {
		require.NoError(t, exec.Command(c[0], c[1:]...).Run())
	}

	// Initial commit on the default branch.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# test"), 0644))
	require.NoError(t, exec.Command("git", "-C", dir, "add", ".").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "-m", "init").Run())

	out, err := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD").Output()
	require.NoError(t, err)
	defaultBranch = strings.TrimSpace(string(out))

	// Create a feature branch with one added file.
	require.NoError(t, exec.Command("git", "-C", dir, "checkout", "-b", "feature").Run())
	require.NoError(t, os.WriteFile(filepath.Join(dir, "new.txt"), []byte("line1\n"), 0644))
	require.NoError(t, exec.Command("git", "-C", dir, "add", ".").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "-m", "add new.txt").Run())

	return dir, defaultBranch
}

func TestComplianceStage_Name(t *testing.T) {
	s := NewComplianceStage(2000, 50)
	assert.Equal(t, "compliance", s.Name())
}

func TestComplianceStage_SkipNoPath(t *testing.T) {
	s := NewComplianceStage(2000, 50)
	state := &PipelineState{Repo: RepoContext{LocalPath: ""}}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusSkipped, result.Status)
}

func TestComplianceStage_Pass(t *testing.T) {
	dir, defaultBranch := initComplianceRepo(t)

	s := NewComplianceStage(2000, 50)
	state := &PipelineState{
		Repo: RepoContext{LocalPath: dir, DefaultBranch: defaultBranch},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusPassed, result.Status)
	require.NotNil(t, state.Compliance)
	assert.True(t, state.Compliance.Passed)
	assert.Equal(t, 1, state.Compliance.FilesChanged)
}

func TestComplianceStage_TooManyFiles(t *testing.T) {
	_, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git binary not available")
	}

	dir, defaultBranch := initComplianceRepo(t)

	// Add more files on the feature branch.
	for i := 0; i < 5; i++ {
		name := filepath.Join(dir, "extra"+string(rune('a'+i))+".txt")
		require.NoError(t, os.WriteFile(name, []byte("x\n"), 0644))
	}
	require.NoError(t, exec.Command("git", "-C", dir, "add", ".").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "-m", "add extras").Run())

	// Limit to 3 files.
	s := NewComplianceStage(2000, 3)
	state := &PipelineState{
		Repo: RepoContext{LocalPath: dir, DefaultBranch: defaultBranch},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	require.NotNil(t, state.Compliance)
	assert.False(t, state.Compliance.Passed)
	assert.NotEmpty(t, state.Compliance.Violations)
}

func TestComplianceStage_TooManyLines(t *testing.T) {
	_, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git binary not available")
	}

	dir, defaultBranch := initComplianceRepo(t)

	// Write a file with many lines on the feature branch.
	var sb strings.Builder
	for i := 0; i < 20; i++ {
		sb.WriteString("line\n")
	}
	require.NoError(t, os.WriteFile(filepath.Join(dir, "big.txt"), []byte(sb.String()), 0644))
	require.NoError(t, exec.Command("git", "-C", dir, "add", ".").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "-m", "add big file").Run())

	// Limit to 5 diff lines.
	s := NewComplianceStage(5, 50)
	state := &PipelineState{
		Repo: RepoContext{LocalPath: dir, DefaultBranch: defaultBranch},
	}

	result, err := s.Run(context.Background(), state)

	require.NoError(t, err)
	assert.Equal(t, StatusFailed, result.Status)
	require.NotNil(t, state.Compliance)
	assert.False(t, state.Compliance.Passed)
	assert.NotEmpty(t, state.Compliance.Violations)
}
