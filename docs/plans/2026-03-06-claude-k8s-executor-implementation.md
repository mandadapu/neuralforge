# Claude Code K8s Executor Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `KubernetesExecutor` that runs Claude Code CLI inside K8s pods to autonomously implement code changes.

**Architecture:** New executor creates K8s Jobs via `client-go`. Each job has an init container (git clone) and main container (claude CLI). Results come back via git push to a feature branch.

**Tech Stack:** Go, `k8s.io/client-go`, `k8s.io/api`, K8s Jobs/Pods API

---

## Task 1: Add K8s Config

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

**Step 1: Write the failing test**

Add to `internal/config/config_test.go`:
```go
func TestLoadFromEnvKubernetes(t *testing.T) {
	t.Setenv("NEURALFORGE_EXECUTOR", "kubernetes")
	t.Setenv("NEURALFORGE_K8S_NAMESPACE", "forge-ns")
	t.Setenv("NEURALFORGE_K8S_IMAGE", "my-image:v1")
	t.Setenv("NEURALFORGE_K8S_SECRET", "my-llm-keys")
	t.Setenv("NEURALFORGE_K8S_GIT_SECRET", "my-git-token")
	t.Setenv("NEURALFORGE_K8S_CPU", "4")
	t.Setenv("NEURALFORGE_K8S_MEMORY", "8Gi")

	cfg := LoadFromEnv()

	assert.Equal(t, "kubernetes", cfg.Executor.DefaultType)
	assert.Equal(t, "forge-ns", cfg.Executor.Kubernetes.Namespace)
	assert.Equal(t, "my-image:v1", cfg.Executor.Kubernetes.Image)
	assert.Equal(t, "my-llm-keys", cfg.Executor.Kubernetes.SecretName)
	assert.Equal(t, "my-git-token", cfg.Executor.Kubernetes.GitSecretName)
	assert.Equal(t, "4", cfg.Executor.Kubernetes.CPU)
	assert.Equal(t, "8Gi", cfg.Executor.Kubernetes.Memory)
}

func TestLoadFromEnvKubernetesDefaults(t *testing.T) {
	// Clear K8s env vars
	for _, k := range []string{"NEURALFORGE_K8S_NAMESPACE", "NEURALFORGE_K8S_IMAGE", "NEURALFORGE_K8S_SECRET", "NEURALFORGE_K8S_GIT_SECRET", "NEURALFORGE_K8S_CPU", "NEURALFORGE_K8S_MEMORY"} {
		os.Unsetenv(k)
	}

	cfg := LoadFromEnv()

	assert.Equal(t, "neuralforge", cfg.Executor.Kubernetes.Namespace)
	assert.Equal(t, "ghcr.io/neuralforge/claude-executor:latest", cfg.Executor.Kubernetes.Image)
	assert.Equal(t, "neuralforge-llm-keys", cfg.Executor.Kubernetes.SecretName)
	assert.Equal(t, "neuralforge-git-token", cfg.Executor.Kubernetes.GitSecretName)
	assert.Equal(t, "2", cfg.Executor.Kubernetes.CPU)
	assert.Equal(t, "4Gi", cfg.Executor.Kubernetes.Memory)
}
```

**Step 2: Run test to verify it fails**

```bash
cd ~/src/neuralforge && go test ./internal/config/ -run TestLoadFromEnvKubernetes -v
```
Expected: FAIL — `cfg.Executor.Kubernetes` undefined

**Step 3: Add KubernetesConfig to config.go**

Add the struct and populate it in `LoadFromEnv`:

```go
// Add to ExecutorConfig:
type ExecutorConfig struct {
	DefaultType string
	Docker      DockerConfig
	Kubernetes  KubernetesConfig
}

// New struct:
type KubernetesConfig struct {
	Namespace     string
	Image         string
	SecretName    string
	GitSecretName string
	Timeout       time.Duration
	CPU           string
	Memory        string
}
```

In `LoadFromEnv`, add inside the `Executor` block:
```go
Kubernetes: KubernetesConfig{
	Namespace:     envStr("NEURALFORGE_K8S_NAMESPACE", "neuralforge"),
	Image:         envStr("NEURALFORGE_K8S_IMAGE", "ghcr.io/neuralforge/claude-executor:latest"),
	SecretName:    envStr("NEURALFORGE_K8S_SECRET", "neuralforge-llm-keys"),
	GitSecretName: envStr("NEURALFORGE_K8S_GIT_SECRET", "neuralforge-git-token"),
	Timeout:       time.Duration(envInt("NEURALFORGE_K8S_TIMEOUT_MINUTES", 30)) * time.Minute,
	CPU:           envStr("NEURALFORGE_K8S_CPU", "2"),
	Memory:        envStr("NEURALFORGE_K8S_MEMORY", "4Gi"),
},
```

**Step 4: Run tests**

```bash
go test ./internal/config/ -v
```
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/config/ && git commit -m "feat: add Kubernetes executor config"
```

---

## Task 2: K8s Executor — Job Builder

**Files:**
- Create: `internal/executor/kubernetes.go`
- Create: `internal/executor/kubernetes_test.go`

**Step 1: Write the test for pod spec building**

Create `internal/executor/kubernetes_test.go` (add to existing file if it exists):
```go
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
		RepoPath: "https://github.com/owner/repo.git",
		Branch:   "neuralforge/issue-42",
		Prompt:   "Fix the login bug",
		Context:  "# Codebase context",
		Timeout:  30 * time.Minute,
	}

	k8sJob := k.buildJobSpec(job)

	// Job metadata
	assert.Equal(t, "neuralforge-job-42", k8sJob.Name)
	assert.Equal(t, "forge-ns", k8sJob.Namespace)

	// Backoff limit = 0 (no K8s retries)
	require.NotNil(t, k8sJob.Spec.BackoffLimit)
	assert.Equal(t, int32(0), *k8sJob.Spec.BackoffLimit)

	// Active deadline
	require.NotNil(t, k8sJob.Spec.ActiveDeadlineSeconds)
	assert.Equal(t, int64(1800), *k8sJob.Spec.ActiveDeadlineSeconds)

	// Init container
	require.Len(t, k8sJob.Spec.Template.Spec.InitContainers, 1)
	init := k8sJob.Spec.Template.Spec.InitContainers[0]
	assert.Equal(t, "git-clone", init.Name)

	// Main container
	require.Len(t, k8sJob.Spec.Template.Spec.Containers, 1)
	main := k8sJob.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "claude-executor", main.Name)
	assert.Equal(t, "claude-exec:v1", main.Image)
	assert.Equal(t, "/workspace", main.WorkingDir)

	// Shared volume
	require.Len(t, k8sJob.Spec.Template.Spec.Volumes, 1)
	assert.Equal(t, "workspace", k8sJob.Spec.Template.Spec.Volumes[0].Name)
}

func TestK8sJobName(t *testing.T) {
	k := &KubernetesExecutor{}

	// Job names must be DNS-safe
	assert.Equal(t, "neuralforge-owner-repo-42", k.jobName("owner/repo#42"))
	assert.Equal(t, "neuralforge-simple", k.jobName("simple"))
}
```

**Step 2: Run test to verify it fails**

```bash
go test ./internal/executor/ -run TestK8s -v
```
Expected: FAIL — `KubernetesExecutor` undefined

**Step 3: Implement K8s executor**

Create `internal/executor/kubernetes.go`:
```go
package executor

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type KubernetesExecutor struct {
	client        kubernetes.Interface
	namespace     string
	image         string
	secretName    string
	gitSecretName string
	cpu           string
	memory        string
}

func NewKubernetes(namespace, image, secretName, gitSecretName, cpu, memory string) (*KubernetesExecutor, error) {
	// Try in-cluster config first, fall back to kubeconfig
	config, err := rest.InClusterConfig()
	if err != nil {
		config, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
		if err != nil {
			return nil, fmt.Errorf("k8s config: %w", err)
		}
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("k8s client: %w", err)
	}

	return &KubernetesExecutor{
		client:        clientset,
		namespace:     namespace,
		image:         image,
		secretName:    secretName,
		gitSecretName: gitSecretName,
		cpu:           cpu,
		memory:        memory,
	}, nil
}

// NewKubernetesWithClient creates executor with a pre-built client (for testing).
func NewKubernetesWithClient(client kubernetes.Interface, namespace, image, secretName, gitSecretName, cpu, memory string) *KubernetesExecutor {
	return &KubernetesExecutor{
		client:        client,
		namespace:     namespace,
		image:         image,
		secretName:    secretName,
		gitSecretName: gitSecretName,
		cpu:           cpu,
		memory:        memory,
	}
}

func (k *KubernetesExecutor) Name() string { return "kubernetes" }

var dnsUnsafe = regexp.MustCompile(`[^a-z0-9-]`)

func (k *KubernetesExecutor) jobName(id string) string {
	name := "neuralforge-" + strings.ToLower(id)
	name = dnsUnsafe.ReplaceAllString(name, "-")
	if len(name) > 63 {
		name = name[:63]
	}
	return strings.TrimRight(name, "-")
}

func (k *KubernetesExecutor) buildJobSpec(job ExecutorJob) *batchv1.Job {
	backoffLimit := int32(0)
	deadlineSeconds := int64(job.Timeout.Seconds())
	name := k.jobName(job.ID)

	// Shell script for main container:
	// 1. Configure git
	// 2. Create branch
	// 3. Run claude CLI with the prompt
	// 4. If changes exist, commit and push
	script := fmt.Sprintf(`#!/bin/sh
set -e
cd /workspace
git config user.email "neuralforge@bot"
git config user.name "NeuralForge"
git checkout -b %s

# Write prompt to temp file
cat > /tmp/prompt.txt << 'PROMPT_EOF'
%s
PROMPT_EOF

# Run Claude Code CLI
claude -p "$(cat /tmp/prompt.txt)" --dangerously-skip-permissions

# Commit and push if there are changes
if [ -n "$(git status --porcelain)" ]; then
  git add -A
  git commit -m "feat: implement issue changes (NeuralForge)"
  git push origin %s
fi
`, job.Branch, job.Prompt, job.Branch)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: k.namespace,
			Labels: map[string]string{
				"app":    "neuralforge",
				"job-id": job.ID,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoffLimit,
			ActiveDeadlineSeconds: &deadlineSeconds,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":    "neuralforge",
						"job-id": job.ID,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:  "git-clone",
							Image: "alpine/git:latest",
							Command: []string{"sh", "-c", fmt.Sprintf(
								"git clone --depth=1 https://x-access-token:$(GIT_TOKEN)@github.com/%s.git /workspace",
								strings.TrimSuffix(job.RepoPath, ".git"),
							)},
							Env: []corev1.EnvVar{
								{
									Name: "GIT_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: k.gitSecretName},
											Key:                  "token",
										},
									},
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workspace", MountPath: "/workspace"},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:       "claude-executor",
							Image:      k.image,
							WorkingDir: "/workspace",
							Command:    []string{"sh", "-c", script},
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{Name: k.secretName},
									},
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "GIT_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: k.gitSecretName},
											Key:                  "token",
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(k.cpu),
									corev1.ResourceMemory: resource.MustParse(k.memory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(k.cpu),
									corev1.ResourceMemory: resource.MustParse(k.memory),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "workspace", MountPath: "/workspace"},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "workspace",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		},
	}
}

func (k *KubernetesExecutor) Run(ctx context.Context, job ExecutorJob) (ExecutorResult, error) {
	jobSpec := k.buildJobSpec(job)

	// Create the K8s Job
	created, err := k.client.BatchV1().Jobs(k.namespace).Create(ctx, jobSpec, metav1.CreateOptions{})
	if err != nil {
		return ExecutorResult{}, fmt.Errorf("create k8s job: %w", err)
	}

	name := created.Name
	slog.Info("k8s job created", "name", name, "namespace", k.namespace)

	// Poll for completion
	result := k.waitForCompletion(ctx, name, job.Timeout)

	// Read logs
	stdout, stderr := k.readLogs(ctx, name)
	result.Stdout = stdout
	result.Stderr = stderr

	return result, nil
}

func (k *KubernetesExecutor) waitForCompletion(ctx context.Context, name string, timeout time.Duration) ExecutorResult {
	deadline := time.After(timeout + 30*time.Second) // extra buffer beyond K8s deadline
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ExecutorResult{TimedOut: true}
		case <-deadline:
			return ExecutorResult{TimedOut: true}
		case <-ticker.C:
			job, err := k.client.BatchV1().Jobs(k.namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				slog.Error("poll k8s job", "error", err)
				continue
			}

			for _, cond := range job.Status.Conditions {
				if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
					return ExecutorResult{Success: true}
				}
				if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
					timedOut := strings.Contains(cond.Reason, "DeadlineExceeded")
					return ExecutorResult{Success: false, TimedOut: timedOut}
				}
			}
		}
	}
}

func (k *KubernetesExecutor) readLogs(ctx context.Context, jobName string) (string, string) {
	// List pods for this job
	pods, err := k.client.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil || len(pods.Items) == 0 {
		return "", ""
	}

	podName := pods.Items[0].Name

	// Read main container logs
	req := k.client.CoreV1().Pods(k.namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: "claude-executor",
	})
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Sprintf("log read error: %v", err)
	}
	defer stream.Close()

	var buf strings.Builder
	b := make([]byte, 4096)
	for {
		n, err := stream.Read(b)
		if n > 0 {
			buf.Write(b[:n])
		}
		if err != nil {
			break
		}
	}

	return buf.String(), ""
}

func (k *KubernetesExecutor) Cleanup(ctx context.Context, jobID string) error {
	name := k.jobName(jobID)
	propagation := metav1.DeletePropagationBackground
	err := k.client.BatchV1().Jobs(k.namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
	if err != nil {
		return fmt.Errorf("delete k8s job %s: %w", name, err)
	}
	return nil
}
```

**Step 4: Install K8s deps and run tests**

```bash
cd ~/src/neuralforge && go get k8s.io/client-go@latest k8s.io/api@latest k8s.io/apimachinery@latest
go mod tidy
go test ./internal/executor/ -v
```
Expected: ALL PASS

**Step 5: Commit**

```bash
git add internal/executor/kubernetes.go internal/executor/kubernetes_test.go go.mod go.sum && git commit -m "feat: add Kubernetes executor with K8s Job builder"
```

---

## Task 3: Wire K8s Executor in App

**Files:**
- Modify: `internal/app/app.go:140-151`

**Step 1: Update buildJobHandler to support kubernetes executor type**

In `buildJobHandler()`, replace the hardcoded Docker executor with a switch:

```go
// Replace line 151:
//   exec := executor.NewDocker(a.cfg.Executor.Docker.Image)
// With:

var exec executor.Executor
switch a.cfg.Executor.DefaultType {
case "kubernetes":
	k8sCfg := a.cfg.Executor.Kubernetes
	var err error
	exec, err = executor.NewKubernetes(
		k8sCfg.Namespace, k8sCfg.Image,
		k8sCfg.SecretName, k8sCfg.GitSecretName,
		k8sCfg.CPU, k8sCfg.Memory,
	)
	if err != nil {
		slog.Error("failed to create k8s executor, falling back to docker", "error", err)
		exec = executor.NewDocker(a.cfg.Executor.Docker.Image)
	}
default:
	exec = executor.NewDocker(a.cfg.Executor.Docker.Image)
}
```

Also update the execute stage timeout to use the right config:
```go
// Replace:
//   pipeline.NewExecuteStage(exec, a.cfg.Executor.Docker.Timeout),
// With:
var execTimeout time.Duration
if a.cfg.Executor.DefaultType == "kubernetes" {
	execTimeout = a.cfg.Executor.Kubernetes.Timeout
} else {
	execTimeout = a.cfg.Executor.Docker.Timeout
}
pipeline.NewExecuteStage(exec, execTimeout),
```

**Step 2: Verify it compiles**

```bash
go build ./...
```

**Step 3: Run all tests**

```bash
go test ./... -count=1
```
Expected: ALL PASS

**Step 4: Commit**

```bash
git add internal/app/app.go && git commit -m "feat: wire Kubernetes executor in app with Docker fallback"
```

---

## Task 4: Claude Executor Dockerfile

**Files:**
- Create: `deploy/claude-executor.Dockerfile`

**Step 1: Create the Dockerfile**

```dockerfile
FROM node:22-slim

# Install system deps
RUN apt-get update && apt-get install -y --no-install-recommends \
    git \
    ca-certificates \
    curl \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code CLI
RUN npm install -g @anthropic-ai/claude-code

# Configure git for push
RUN git config --global user.email "neuralforge@bot" && \
    git config --global user.name "NeuralForge"

WORKDIR /workspace

# Default entrypoint — overridden by K8s Job spec
ENTRYPOINT ["claude"]
```

**Step 2: Commit**

```bash
git add deploy/ && git commit -m "feat: add Claude executor Docker image"
```

---

## Task 5: K8s Secrets Example + Docs

**Files:**
- Create: `deploy/k8s-secrets.yaml.example`

**Step 1: Create example secrets manifest**

```yaml
# LLM API keys — supports ANTHROPIC_API_KEY or OAuth credentials
apiVersion: v1
kind: Secret
metadata:
  name: neuralforge-llm-keys
  namespace: neuralforge
type: Opaque
stringData:
  # Option A: API key auth
  ANTHROPIC_API_KEY: "sk-ant-..."

  # Option B: OAuth credentials (comment out API key above)
  # ANTHROPIC_AUTH_TOKEN: "oauth-token-..."
  # ANTHROPIC_AUTH_REFRESH_TOKEN: "refresh-token-..."

---
# Git access token for clone + push
apiVersion: v1
kind: Secret
metadata:
  name: neuralforge-git-token
  namespace: neuralforge
type: Opaque
stringData:
  token: "ghp_..."
```

**Step 2: Commit**

```bash
git add deploy/ && git commit -m "docs: add K8s secrets example manifest"
```

---

## Summary

| Task | What | Files |
|------|------|-------|
| 1 | K8s config (env vars + struct) | `config.go`, `config_test.go` |
| 2 | K8s executor (Job builder + poll + logs) | `kubernetes.go`, `kubernetes_test.go` |
| 3 | Wire executor in app | `app.go` |
| 4 | Claude executor Dockerfile | `deploy/claude-executor.Dockerfile` |
| 5 | K8s secrets example | `deploy/k8s-secrets.yaml.example` |

**Total: 5 tasks, ~25 steps**
