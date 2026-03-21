package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// validateLocalPath ensures a filesystem path is absolute and clean,
// preventing path traversal attacks in Docker volume mounts.
func validateLocalPath(path string) error {
	if path == "" {
		return fmt.Errorf("path must not be empty")
	}
	if !filepath.IsAbs(filepath.Clean(path)) {
		return fmt.Errorf("path must be absolute: %s", path)
	}
	return nil
}

type DockerExecutor struct {
	image string
}

func NewDocker(image string) *DockerExecutor {
	return &DockerExecutor{image: image}
}

func (d *DockerExecutor) Name() string { return "docker" }

func (d *DockerExecutor) buildArgs(job ExecutorJob) []string {
	args := []string{
		"run", "--rm",
		"-v", fmt.Sprintf("%s:/workspace", job.RepoPath),
		"-w", "/workspace",
		"-e", fmt.Sprintf("BRANCH=%s", job.Branch),
		"-e", fmt.Sprintf("JOB_ID=%s", job.ID),
	}
	for k, v := range job.EnvVars {
		args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, d.image)
	return args
}

func (d *DockerExecutor) Run(ctx context.Context, job ExecutorJob) (ExecutorResult, error) {
	if err := validateBranch(job.Branch); err != nil {
		return ExecutorResult{}, fmt.Errorf("invalid branch: %w", err)
	}
	if err := validateLocalPath(job.RepoPath); err != nil {
		return ExecutorResult{}, fmt.Errorf("invalid repo path: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, job.Timeout)
	defer cancel()

	args := d.buildArgs(job)
	cmd := exec.CommandContext(ctx, "docker", args...)
	cmd.Stdin = strings.NewReader(job.Prompt)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	timedOut := ctx.Err() != nil

	return ExecutorResult{
		Success:  err == nil,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		TimedOut: timedOut,
	}, nil
}

func (d *DockerExecutor) Cleanup(ctx context.Context, jobID string) error {
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", fmt.Sprintf("nf-%s", jobID))
	return cmd.Run()
}
