package executor

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
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

	require.Len(t, k8sJob.Spec.Template.Spec.InitContainers, 1)
	assert.Equal(t, "git-clone", k8sJob.Spec.Template.Spec.InitContainers[0].Name)

	require.Len(t, k8sJob.Spec.Template.Spec.Containers, 1)
	main := k8sJob.Spec.Template.Spec.Containers[0]
	assert.Equal(t, "claude-executor", main.Name)
	assert.Equal(t, "claude-exec:v1", main.Image)
	assert.Equal(t, "/workspace", main.WorkingDir)

	require.Len(t, k8sJob.Spec.Template.Spec.Volumes, 1)
	assert.Equal(t, "workspace", k8sJob.Spec.Template.Spec.Volumes[0].Name)
}

func TestK8sJobName(t *testing.T) {
	k := &KubernetesExecutor{}
	assert.Equal(t, "neuralforge-owner-repo-42", k.jobName("owner/repo#42"))
	assert.Equal(t, "neuralforge-simple", k.jobName("simple"))
}

func TestReadLogsAfterPodTermination(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "neuralforge-job-1-abc",
			Namespace: "test-ns",
			Labels:    map[string]string{"job-name": "neuralforge-job-1"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
		},
	}

	client := fake.NewSimpleClientset(pod)
	k := NewKubernetesWithClient(client, "test-ns", "img", "secret", "git-secret", "1", "1Gi")

	ctx := context.Background()

	// waitForPodTermination should return immediately since pod is already Succeeded
	resultPod, err := k.waitForPodTermination(ctx, "neuralforge-job-1", 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "neuralforge-job-1-abc", resultPod.Name)
	assert.Equal(t, corev1.PodSucceeded, resultPod.Status.Phase)

	// readLogs on the fake client will return an error (fake doesn't support log streaming),
	// but the retry logic should exhaust attempts and return the error in stderr.
	stdout, stderr := k.readLogs(ctx, resultPod.Name)
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "failed to read logs after 3 attempts")
}

func TestReadLogsRetryOnStreamError(t *testing.T) {
	// The fake client doesn't support log streaming, so every attempt will fail.
	// This test verifies the retry logic exhausts all 3 attempts and returns
	// a descriptive error message.
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-pod",
			Namespace: "test-ns",
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodSucceeded,
		},
	}

	client := fake.NewSimpleClientset(pod)
	k := NewKubernetesWithClient(client, "test-ns", "img", "secret", "git-secret", "1", "1Gi")

	ctx := context.Background()
	stdout, stderr := k.readLogs(ctx, "test-pod")
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "failed to read logs after 3 attempts")
}

func TestWaitForPodTerminationTimeout(t *testing.T) {
	// Pod stuck in Running — should timeout
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "stuck-pod",
			Namespace: "test-ns",
			Labels:    map[string]string{"job-name": "neuralforge-stuck"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	client := fake.NewSimpleClientset(pod)
	k := NewKubernetesWithClient(client, "test-ns", "img", "secret", "git-secret", "1", "1Gi")

	ctx := context.Background()
	// Use a short timeout to keep the test fast
	_, err := k.waitForPodTermination(ctx, "neuralforge-stuck", 3*time.Second)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "timed out waiting for pod termination")
}

func TestWaitForPodTerminationFailedPhase(t *testing.T) {
	// Pod in Failed phase should return immediately
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "failed-pod",
			Namespace: "test-ns",
			Labels:    map[string]string{"job-name": "neuralforge-failed"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodFailed,
		},
	}

	client := fake.NewSimpleClientset(pod)
	k := NewKubernetesWithClient(client, "test-ns", "img", "secret", "git-secret", "1", "1Gi")

	ctx := context.Background()
	resultPod, err := k.waitForPodTermination(ctx, "neuralforge-failed", 5*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "failed-pod", resultPod.Name)
	assert.Equal(t, corev1.PodFailed, resultPod.Status.Phase)
}

func TestWaitForPodTerminationContextCancelled(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "running-pod",
			Namespace: "test-ns",
			Labels:    map[string]string{"job-name": "neuralforge-running"},
		},
		Status: corev1.PodStatus{
			Phase: corev1.PodRunning,
		},
	}

	client := fake.NewSimpleClientset(pod)
	k := NewKubernetesWithClient(client, "test-ns", "img", "secret", "git-secret", "1", "1Gi")

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, err := k.waitForPodTermination(ctx, "neuralforge-running", 30*time.Second)
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled)
}
