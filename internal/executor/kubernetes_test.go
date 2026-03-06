package executor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestK8sJobSpec(t *testing.T) {
	k := &KubernetesExecutor{
		namespace:     "forge-ns",
		image:         "claude-exec:v1",
		secretName:    "llm-keys",
		gitSecretName: "git-token",
		cpu:           "2",
		memory:        "4Gi",
	}

	job := ExecutorJob{
		ID:       "job-42",
		RepoPath: "owner/repo",
		Branch:   "neuralforge/issue-42",
		Prompt:   "Fix the login bug",
		Context:  "# Codebase context",
		Timeout:  30 * time.Minute,
	}

	k8sJob, err := k.buildJobSpec(job)
	require.NoError(t, err)

	assert.Equal(t, "neuralforge-job-42", k8sJob.Name)
	assert.Equal(t, "forge-ns", k8sJob.Namespace)

	require.NotNil(t, k8sJob.Spec.BackoffLimit)
	assert.Equal(t, int32(0), *k8sJob.Spec.BackoffLimit)

	require.NotNil(t, k8sJob.Spec.ActiveDeadlineSeconds)
	assert.Equal(t, int64(1800), *k8sJob.Spec.ActiveDeadlineSeconds)

	require.Len(t, k8sJob.Spec.Template.Spec.InitContainers, 1)
	assert.Equal(t, "git-clone", k8sJob.Spec.Template.Spec.InitContainers[0].Name)

	require.Len(t, k8sJob.Spec.Template.Spec.Containers, 1)
	main := k8sJob.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "claude-executor", main.Name)
	assert.Equal(t, "claude-exec:v1", main.Image)
	assert.Equal(t, "/workspace", main.WorkingDir)

	require.Len(t, k8sJob.Spec.Template.Spec.Volumes, 1)
	assert.Equal(t, "workspace", k8sJob.Spec.Template.Spec.Volumes[0].Name)

	// Verify label is sanitized
	assert.Equal(t, "neuralforge-job-42", k8sJob.Labels["job-id"])
}

func TestK8sJobName(t *testing.T) {
	k := &KubernetesExecutor{}
	assert.Equal(t, "neuralforge-owner-repo-42", k.jobName("owner/repo#42"))
	assert.Equal(t, "neuralforge-simple", k.jobName("simple"))
}

func TestBuildJobSpec_InvalidRepoPath(t *testing.T) {
	k := &KubernetesExecutor{
		namespace:     "forge-ns",
		image:         "claude-exec:v1",
		secretName:    "llm-keys",
		gitSecretName: "git-token",
		cpu:           "2",
		memory:        "4Gi",
	}

	job := ExecutorJob{
		ID:       "job-1",
		RepoPath: "$(evil)",
		Branch:   "neuralforge/issue-1",
		Prompt:   "Do something",
		Timeout:  10 * time.Minute,
	}

	result, err := k.buildJobSpec(job)
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsafe repo path")
}

func TestBuildJobSpec_InvalidBranch(t *testing.T) {
	k := &KubernetesExecutor{
		namespace:     "forge-ns",
		image:         "claude-exec:v1",
		secretName:    "llm-keys",
		gitSecretName: "git-token",
		cpu:           "2",
		memory:        "4Gi",
	}

	job := ExecutorJob{
		ID:       "job-1",
		RepoPath: "owner/repo",
		Branch:   "; rm -rf /",
		Prompt:   "Do something",
		Timeout:  10 * time.Minute,
	}

	result, err := k.buildJobSpec(job)
	assert.Nil(t, result)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsafe branch name")
}
