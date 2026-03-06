package executor

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/mandadapu/neuralforge/internal/validate"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// KubernetesExecutor runs jobs as Kubernetes Jobs using client-go.
type KubernetesExecutor struct {
	client        kubernetes.Interface
	namespace     string
	image         string
	secretName    string
	gitSecretName string
	cpu           string
	memory        string
}

// NewKubernetes creates a KubernetesExecutor, trying in-cluster config first,
// then falling back to the default kubeconfig.
func NewKubernetes(namespace, image, secretName, gitSecretName, cpu, memory string) (*KubernetesExecutor, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		// Fall back to kubeconfig
		kubeconfig := homedir.HomeDir() + "/.kube/config"
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, fmt.Errorf("failed to build k8s config: %w", err)
		}
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %w", err)
	}

	return &KubernetesExecutor{
		client:        client,
		namespace:     namespace,
		image:         image,
		secretName:    secretName,
		gitSecretName: gitSecretName,
		cpu:           cpu,
		memory:        memory,
	}, nil
}

// NewKubernetesWithClient creates a KubernetesExecutor with the provided client,
// useful for testing with a mock/fake client.
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

// jobName converts a job ID into a DNS-safe Kubernetes job name.
// Lowercase, non-alphanumeric chars replaced with "-", max 63 chars.
func (k *KubernetesExecutor) jobName(id string) string {
	name := "neuralforge-" + strings.ToLower(id)
	name = nonAlphanumeric.ReplaceAllString(name, "-")
	name = strings.Trim(name, "-")
	if len(name) > 63 {
		name = name[:63]
		name = strings.TrimRight(name, "-")
	}
	return name
}

// buildJobSpec constructs the Kubernetes Job spec for the given ExecutorJob.
func (k *KubernetesExecutor) buildJobSpec(job ExecutorJob) (*batchv1.Job, error) {
	if err := validate.RepoFullName(job.RepoPath); err != nil {
		return nil, fmt.Errorf("unsafe repo path: %w", err)
	}
	if err := validate.BranchName(job.Branch); err != nil {
		return nil, fmt.Errorf("unsafe branch name: %w", err)
	}

	backoffLimit := int32(0)
	deadlineSeconds := int64(job.Timeout.Seconds())

	// Shell script for the main container: configure git, create branch,
	// run claude, commit and push if changes exist.
	script := fmt.Sprintf(`set -e
git config user.email "neuralforge@bot"
git config user.name "NeuralForge"
git checkout -b %s
claude -p %q --dangerously-skip-permissions
if [ -n "$(git status --porcelain)" ]; then
  git add -A
  git commit -m "neuralforge: apply changes"
  git push origin %s
fi
`, job.Branch, job.Prompt, job.Branch)

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      k.jobName(job.ID),
			Namespace: k.namespace,
			Labels: map[string]string{
				"app":    "neuralforge",
				"job-id": k.jobName(job.ID),
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:          &backoffLimit,
			ActiveDeadlineSeconds: &deadlineSeconds,
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:  "git-clone",
							Image: "alpine/git",
							Command: []string{"sh", "-c", fmt.Sprintf(
								`git clone https://x-access-token:$(GIT_TOKEN)@github.com/%s.git /workspace`,
								job.RepoPath,
							)},
							Env: []corev1.EnvVar{
								{
									Name: "GIT_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: k.gitSecretName,
											},
											Key: "token",
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
										LocalObjectReference: corev1.LocalObjectReference{
											Name: k.secretName,
										},
									},
								},
							},
							Env: []corev1.EnvVar{
								{
									Name: "GIT_TOKEN",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: k.gitSecretName,
											},
											Key: "token",
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
	}, nil
}

// Run creates a Kubernetes Job, waits for completion, reads logs, and returns
// the result.
func (k *KubernetesExecutor) Run(ctx context.Context, job ExecutorJob) (ExecutorResult, error) {
	k8sJob, err := k.buildJobSpec(job)
	if err != nil {
		return ExecutorResult{}, fmt.Errorf("build job spec: %w", err)
	}

	created, err := k.client.BatchV1().Jobs(k.namespace).Create(ctx, k8sJob, metav1.CreateOptions{})
	if err != nil {
		return ExecutorResult{}, fmt.Errorf("failed to create k8s job: %w", err)
	}

	name := created.Name
	success, timedOut, err := k.waitForCompletion(ctx, name, job.Timeout)
	if err != nil {
		return ExecutorResult{TimedOut: timedOut}, fmt.Errorf("error waiting for job: %w", err)
	}

	stdout, stderr := k.readLogs(ctx, name)

	return ExecutorResult{
		Success:  success,
		Stdout:   stdout,
		Stderr:   stderr,
		TimedOut: timedOut,
	}, nil
}

// waitForCompletion polls the job status every 5 seconds until completion,
// failure, or timeout.
func (k *KubernetesExecutor) waitForCompletion(ctx context.Context, name string, timeout time.Duration) (success bool, timedOut bool, err error) {
	deadline := time.After(timeout)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return false, true, ctx.Err()
		case <-deadline:
			return false, true, fmt.Errorf("job %s timed out after %s", name, timeout)
		case <-ticker.C:
			job, err := k.client.BatchV1().Jobs(k.namespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, false, fmt.Errorf("failed to get job status: %w", err)
			}
			for _, cond := range job.Status.Conditions {
				if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
					return true, false, nil
				}
				if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
					return false, false, nil
				}
			}
		}
	}
}

// readLogs finds the pod for the given job and reads the main container logs.
func (k *KubernetesExecutor) readLogs(ctx context.Context, jobName string) (stdout string, stderr string) {
	pods, err := k.client.CoreV1().Pods(k.namespace).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("job-name=%s", jobName),
	})
	if err != nil || len(pods.Items) == 0 {
		return "", fmt.Sprintf("failed to list pods for job %s: %v", jobName, err)
	}

	podName := pods.Items[0].Name
	container := "claude-executor"
	req := k.client.CoreV1().Pods(k.namespace).GetLogs(podName, &corev1.PodLogOptions{
		Container: container,
	})

	stream, err := req.Stream(ctx)
	if err != nil {
		return "", fmt.Sprintf("failed to stream logs: %v", err)
	}
	defer stream.Close()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Sprintf("failed to read logs: %v", err)
	}

	return string(data), ""
}

// Cleanup deletes the Kubernetes Job and its pods using background propagation.
func (k *KubernetesExecutor) Cleanup(ctx context.Context, jobID string) error {
	name := k.jobName(jobID)
	propagation := metav1.DeletePropagationBackground
	return k.client.BatchV1().Jobs(k.namespace).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &propagation,
	})
}
