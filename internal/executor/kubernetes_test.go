package executor

import (
	"strings"
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

	k8sJob := k.buildJobSpec(job)

	assert.Equal(t, "neuralforge-job-42", k8sJob.Name)
	assert.Equal(t, "forge-ns", k8sJob.Namespace)

	require.NotNil(t, k8sJob.Spec.BackoffLimit)
	assert.Equal(t, int32(0), *k8sJob.Spec.BackoffLimit)

	require.NotNil(t, k8sJob.Spec.ActiveDeadlineSeconds)
	assert.Equal(t, int64(1800), *k8sJob.Spec.ActiveDeadlineSeconds)

	// Init container checks
	require.Len(t, k8sJob.Spec.Template.Spec.InitContainers, 1)
	initContainer := k8sJob.Spec.Template.Spec.InitContainers[0]
	assert.Equal(t, "git-clone", initContainer.Name)

	// Init container should use env var for REPO_PATH, not interpolation
	initCmd := strings.Join(initContainer.Command, " ")
	assert.NotContains(t, initCmd, "owner/repo", "init container command should not contain interpolated repo path")
	assert.Contains(t, initCmd, "${REPO_PATH}", "init container should reference REPO_PATH env var")

	// Verify REPO_PATH env var is set on init container
	var foundRepoPath bool
	for _, env := range initContainer.Env {
		if env.Name == "REPO_PATH" {
			foundRepoPath = true
			assert.Equal(t, "owner/repo", env.Value)
		}
	}
	assert.True(t, foundRepoPath, "init container should have REPO_PATH env var")

	// Main container checks
	require.Len(t, k8sJob.Spec.Template.Spec.Containers, 1)
	main := k8sJob.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "claude-executor", main.Name)
	assert.Equal(t, "claude-exec:v1", main.Image)
	assert.Equal(t, "/workspace", main.WorkingDir)

	// Main container script should use env vars, not interpolated values
	mainCmd := strings.Join(main.Command, " ")
	assert.NotContains(t, mainCmd, "neuralforge/issue-42", "main container command should not contain interpolated branch")
	assert.NotContains(t, mainCmd, "Fix the login bug", "main container command should not contain interpolated prompt")
	assert.Contains(t, mainCmd, "$BRANCH", "main container should reference BRANCH env var")
	assert.Contains(t, mainCmd, "/etc/neuralforge/prompt", "main container should read prompt from ConfigMap mount")

	// Verify BRANCH env var is set on main container
	var foundBranch bool
	for _, env := range main.Env {
		if env.Name == "BRANCH" {
			foundBranch = true
			assert.Equal(t, "neuralforge/issue-42", env.Value)
		}
	}
	assert.True(t, foundBranch, "main container should have BRANCH env var")

	// Volume mount checks
	require.Len(t, main.VolumeMounts, 2)
	assert.Equal(t, "workspace", main.VolumeMounts[0].Name)
	assert.Equal(t, "prompt", main.VolumeMounts[1].Name)
	assert.Equal(t, "/etc/neuralforge", main.VolumeMounts[1].MountPath)
	assert.True(t, main.VolumeMounts[1].ReadOnly)

	// Volume checks
	require.Len(t, k8sJob.Spec.Template.Spec.Volumes, 2)
	assert.Equal(t, "workspace", k8sJob.Spec.Template.Spec.Volumes[0].Name)
	assert.Equal(t, "prompt", k8sJob.Spec.Template.Spec.Volumes[1].Name)
	require.NotNil(t, k8sJob.Spec.Template.Spec.Volumes[1].ConfigMap)
	assert.Equal(t, k.promptConfigMapName("job-42"), k8sJob.Spec.Template.Spec.Volumes[1].ConfigMap.Name)
}

func TestK8sJobName(t *testing.T) {
	k := &KubernetesExecutor{}
	assert.Equal(t, "neuralforge-owner-repo-42", k.jobName("owner/repo#42"))
	assert.Equal(t, "neuralforge-simple", k.jobName("simple"))
}

func TestK8sValidateBranch(t *testing.T) {
	tests := []struct {
		name    string
		branch  string
		wantErr bool
	}{
		{name: "valid simple", branch: "main", wantErr: false},
		{name: "valid with slash", branch: "neuralforge/issue-42", wantErr: false},
		{name: "valid with dot", branch: "fix/my-branch.v2", wantErr: false},
		{name: "valid with underscore", branch: "feature/my_branch", wantErr: false},
		{name: "injection semicolon", branch: "; rm -rf /", wantErr: true},
		{name: "injection command sub", branch: "$(malicious)", wantErr: true},
		{name: "injection backtick", branch: "`whoami`", wantErr: true},
		{name: "empty string", branch: "", wantErr: true},
		{name: "spaces", branch: "has space", wantErr: true},
		{name: "newline", branch: "branch\nname", wantErr: true},
		{name: "pipe", branch: "branch|cat", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBranch(tt.branch)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestK8sValidateRepoPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{name: "valid", path: "owner/repo", wantErr: false},
		{name: "valid with dot", path: "my-org/my-repo.go", wantErr: false},
		{name: "valid with dash", path: "my-org/my-repo", wantErr: false},
		{name: "injection semicolon", path: "owner/repo; echo pwned", wantErr: true},
		{name: "path traversal", path: "../../etc/passwd", wantErr: true},
		{name: "empty string", path: "", wantErr: true},
		{name: "no slash", path: "noslash", wantErr: true},
		{name: "triple path", path: "a/b/c", wantErr: true},
		{name: "backtick", path: "owner/`whoami`", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRepoPath(tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestK8sJobSpecNoInjection(t *testing.T) {
	k := &KubernetesExecutor{
		namespace:     "forge-ns",
		image:         "claude-exec:v1",
		secretName:    "llm-keys",
		gitSecretName: "git-token",
		cpu:           "2",
		memory:        "4Gi",
	}

	// Use malicious values — buildJobSpec itself doesn't validate,
	// but the script should never contain these as literal shell code.
	job := ExecutorJob{
		ID:       "job-evil",
		RepoPath: "owner/repo",
		Branch:   "$(whoami)",
		Prompt:   `"; rm -rf / #`,
		Context:  "",
		Timeout:  10 * time.Minute,
	}

	k8sJob := k.buildJobSpec(job)

	// The shell script in the main container must NOT contain the malicious values inline
	mainCmd := strings.Join(k8sJob.Spec.Template.Spec.Containers[0].Command, " ")
	assert.NotContains(t, mainCmd, "$(whoami)", "branch injection must not appear in shell script")
	assert.NotContains(t, mainCmd, "rm -rf", "prompt injection must not appear in shell script")

	// The init container command must NOT contain interpolated repo path
	initCmd := strings.Join(k8sJob.Spec.Template.Spec.InitContainers[0].Command, " ")
	assert.NotContains(t, initCmd, "owner/repo", "repo path must not be interpolated in init container command")

	// Values should be in env vars instead
	var branchEnv string
	for _, env := range k8sJob.Spec.Template.Spec.Containers[0].Env {
		if env.Name == "BRANCH" {
			branchEnv = env.Value
		}
	}
	assert.Equal(t, "$(whoami)", branchEnv, "malicious branch should be safely stored in env var")

	var repoPathEnv string
	for _, env := range k8sJob.Spec.Template.Spec.InitContainers[0].Env {
		if env.Name == "REPO_PATH" {
			repoPathEnv = env.Value
		}
	}
	assert.Equal(t, "owner/repo", repoPathEnv, "repo path should be in env var")
}

func TestK8sPromptConfigMapName(t *testing.T) {
	k := &KubernetesExecutor{}

	// Deterministic
	assert.Equal(t, k.promptConfigMapName("job-42"), k.promptConfigMapName("job-42"))

	// DNS-safe: lowercase, no special chars
	name := k.promptConfigMapName("Owner/Repo#42")
	assert.Equal(t, name, strings.ToLower(name), "ConfigMap name should be lowercase")
	assert.NotContains(t, name, "/", "ConfigMap name should not contain /")
	assert.NotContains(t, name, "#", "ConfigMap name should not contain #")

	// Max 63 chars
	longID := strings.Repeat("a", 100)
	longName := k.promptConfigMapName(longID)
	assert.LessOrEqual(t, len(longName), 63, "ConfigMap name must not exceed 63 chars")

	// Prefix
	assert.True(t, strings.HasPrefix(k.promptConfigMapName("test-id"), "neuralforge-prompt-"))
}
