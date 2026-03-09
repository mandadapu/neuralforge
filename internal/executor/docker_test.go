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

func TestValidateLocalPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"valid absolute path", "/tmp/repo", false},
		{"valid nested path", "/home/user/projects/myrepo", false},
		{"relative path", "tmp/repo", true},
		{"path with colon (docker injection)", "/tmp/repo:/etc", true},
		{"path with traversal", "/tmp/../etc/passwd", true},
		{"current dir", ".", true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateLocalPath(tc.path)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
