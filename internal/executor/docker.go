package executor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
)

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
	if err := cmd.Run(); err != nil {
		slog.Error("docker cleanup failed", "job_id", jobID, "error", err)
		return fmt.Errorf("docker cleanup: %w", err)
	}
	slog.Info("docker cleanup completed", "job_id", jobID)
	return nil
}
