package executor

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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

// testK8sWithServer creates a KubernetesExecutor backed by an httptest server.
func testK8sWithServer(t *testing.T, handler http.Handler) *KubernetesExecutor {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	client, err := kubernetes.NewForConfig(&rest.Config{Host: server.URL})
	require.NoError(t, err)
	return NewKubernetesWithClient(client, "test-ns", "test-image", "secret", "git-secret", "1", "1Gi")
}

// podListResponse returns a JSON-encoded PodList with the given pods.
func podListResponse(w http.ResponseWriter, pods ...corev1.Pod) {
	resp := corev1.PodList{
		TypeMeta: metav1.TypeMeta{Kind: "PodList", APIVersion: "v1"},
		Items:    pods,
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func TestK8sReadLogs_PodListError(t *testing.T) {
	k := testK8sWithServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(metav1.Status{
			TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"},
			Status:   "Failure",
			Message:  "internal error",
			Code:     500,
		})
	}))

	stdout, err := k.readLogs(context.Background(), "test-job")
	assert.Empty(t, stdout)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to list pods")
}

func TestK8sReadLogs_EmptyPodList(t *testing.T) {
	k := testK8sWithServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		podListResponse(w) // empty list
	}))

	stdout, err := k.readLogs(context.Background(), "test-job")
	assert.Empty(t, stdout)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "no pods found")
}

func TestK8sReadLogs_StreamError(t *testing.T) {
	k := testK8sWithServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/log") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(metav1.Status{
				TypeMeta: metav1.TypeMeta{Kind: "Status", APIVersion: "v1"},
				Status:   "Failure",
				Message:  "log stream error",
				Code:     500,
			})
			return
		}
		podListResponse(w, corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"},
		})
	}))

	stdout, err := k.readLogs(context.Background(), "test-job")
	assert.Empty(t, stdout)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to stream logs")
}

func TestK8sReadLogs_Success(t *testing.T) {
	k := testK8sWithServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/log") {
			w.Header().Set("Content-Type", "text/plain")
			w.Write([]byte("hello from the pod\n"))
			return
		}
		podListResponse(w, corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "test-pod", Namespace: "test-ns"},
		})
	}))

	stdout, err := k.readLogs(context.Background(), "test-job")
	assert.NoError(t, err)
	assert.Equal(t, "hello from the pod\n", stdout)
}

func TestK8sReadLogs_ContextCancelled(t *testing.T) {
	k := testK8sWithServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with cancelled context")
	}))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	stdout, err := k.readLogs(ctx, "test-job")
	assert.Empty(t, stdout)
	assert.Error(t, err)
}
