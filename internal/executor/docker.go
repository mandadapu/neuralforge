package executor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

type DockerExecutor struct {
	image string
}

func NewDocker(image string) *DockerExecutor {
	return &DockerExecutor{image: image}
}

func (d *DockerExecutor) Name() string { return "docker" }

// validateLocalPath checks that a path is absolute and clean to prevent
// path traversal or Docker volume-spec injection (colon is the separator).
func validateLocalPath(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("repo path must be absolute: %q", path)
	}
	if strings.Contains(path, ":") {
		return fmt.Errorf("repo path must not contain ':': %q", path)
	}
	if path != filepath.Clean(path) {
		return fmt.Errorf("repo path must be clean (no traversal): %q", path)
	}
	return nil
}

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
	if err := validateLocalPath(job.RepoPath); err != nil {
		return ExecutorResult{}, fmt.Errorf("invalid repo path: %w", err)
	}
	if err := validateBranch(job.Branch); err != nil {
		return ExecutorResult{}, fmt.Errorf("invalid branch: %w", err)
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
