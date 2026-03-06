package executor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDockerJobArgs(t *testing.T) {
	d := NewDocker("ghcr.io/test:latest")

	job := ExecutorJob{
		ID:       "test-1",
		RepoPath: "/tmp/repo",
		Branch:   "fix-1",
		Prompt:   "implement changes",
		Timeout:  10 * time.Minute,
	}

	args := d.buildArgs(job)
	assert.Contains(t, args, "--rm")
	assert.Contains(t, args, "ghcr.io/test:latest")
	assert.Contains(t, args, "-w")
}
