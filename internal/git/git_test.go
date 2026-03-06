package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func initTestRepo(t *testing.T) string {
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
	f := filepath.Join(dir, "README.md")
	require.NoError(t, os.WriteFile(f, []byte("# Test"), 0644))
	require.NoError(t, exec.Command("git", "-C", dir, "add", ".").Run())
	require.NoError(t, exec.Command("git", "-C", dir, "commit", "-m", "init").Run())
	return dir
}

func TestCreateBranch(t *testing.T) {
	repo := initTestRepo(t)
	g := New(repo)

	err := g.CreateBranch("feature-1")
	require.NoError(t, err)

	branch, err := g.CurrentBranch()
	require.NoError(t, err)
	assert.Equal(t, "feature-1", branch)
}

func TestCommitAndLog(t *testing.T) {
	repo := initTestRepo(t)
	g := New(repo)

	require.NoError(t, g.CreateBranch("test-branch"))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "new.txt"), []byte("hello"), 0644))
	require.NoError(t, g.AddAll())
	require.NoError(t, g.Commit("add new file"))

	log, err := g.Log(1)
	require.NoError(t, err)
	assert.Contains(t, log, "add new file")
}

func TestDiffStat(t *testing.T) {
	repo := initTestRepo(t)
	g := New(repo)

	// Get the default branch name (could be main or master)
	defaultBranch, err := g.CurrentBranch()
	require.NoError(t, err)

	require.NoError(t, g.CreateBranch("diff-branch"))
	require.NoError(t, os.WriteFile(filepath.Join(repo, "a.txt"), []byte("content"), 0644))
	require.NoError(t, g.AddAll())
	require.NoError(t, g.Commit("add a"))

	stat, err := g.DiffStat(defaultBranch)
	require.NoError(t, err)
	assert.Contains(t, stat, "a.txt")
}
