package executor

import (
	"context"
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

func TestDockerCleanup(t *testing.T) {
	d := NewDocker("ghcr.io/test:latest")

	tests := []struct {
		name  string
		jobID string
	}{
		{name: "nonexistent container", jobID: "nonexistent-job-123"},
		{name: "empty job id", jobID: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := d.Cleanup(ctx, tt.jobID)
			// docker may or may not be available in the test environment;
			// either way we exercise the code path and verify it returns an error
			// (docker not installed) or nil (docker present, container not found
			// is not an error for "docker rm -f").
			if err != nil {
				assert.Contains(t, err.Error(), "docker cleanup")
			}
		})
	}
}
