package runtime

import (
	"context"
	"fmt"
	"io"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Workload is an Atlas-managed Deployment in a project namespace (app or dependency).
type Workload struct {
	Name      string
	Component string
	Type      string
	Ready     bool
	Replicas  int32
}

// LogStream is a follow stream for one workload's pod.
type LogStream struct {
	Pod       string
	Container string
	Stream    io.ReadCloser
}

// ListManagedWorkloads lists Atlas-managed Deployments in a project namespace.
func (c *Client) ListManagedWorkloads(ctx context.Context, namespace, projectID string) ([]Workload, error) {
	if namespace == "" {
		namespace = "default"
	}
	selector := fmt.Sprintf("%s=%s", LabelManagedBy, LabelManagedByValue)
	if projectID != "" {
		selector += fmt.Sprintf(",%s=%s", LabelProjectID, projectID)
	}
	list, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("list managed workloads: %w", err)
	}

	out := make([]Workload, 0, len(list.Items))
	for _, d := range list.Items {
		name := d.Labels["app"]
		if name == "" {
			name = d.Name
		}
		w := Workload{
			Name:      name,
			Component: d.Labels[LabelComponent],
			Type:      d.Labels[LabelDepType],
			Ready:     deploymentReady(d),
			Replicas:  d.Status.ReadyReplicas,
		}
		out = append(out, w)
	}
	sort.Slice(out, func(i, j int) bool {
		// Application first, then deps by name.
		if out[i].Component != out[j].Component {
			return out[i].Component == ComponentApplication
		}
		return out[i].Name < out[j].Name
	})
	return out, nil
}

func deploymentReady(d appsv1.Deployment) bool {
	desired := int32(1)
	if d.Spec.Replicas != nil {
		desired = *d.Spec.Replicas
	}
	return desired > 0 && d.Status.ReadyReplicas >= desired
}

// FollowWorkloadLogs streams logs from the best Running pod for workload name (label app=<name>).
func (c *Client) FollowWorkloadLogs(ctx context.Context, namespace, name string, tailLines int64) (*LogStream, error) {
	if name == "" {
		return nil, fmt.Errorf("follow logs: workload name is required")
	}
	if namespace == "" {
		namespace = "default"
	}
	if tailLines <= 0 {
		tailLines = 200
	}

	selector := fmt.Sprintf("app=%s", name)
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("list pods for %s: %w", name, err)
	}
	pod := pickPodForLogs(pods.Items)
	if pod == nil {
		return nil, fmt.Errorf("no pods found for workload %q", name)
	}

	container := name
	if len(pod.Spec.Containers) > 0 {
		found := false
		for _, ctn := range pod.Spec.Containers {
			if ctn.Name == name {
				found = true
				break
			}
		}
		if !found {
			container = pod.Spec.Containers[0].Name
		}
	}

	opts := &corev1.PodLogOptions{
		Container:  container,
		Follow:     true,
		TailLines:  &tailLines,
		Timestamps: false,
	}
	req := c.clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("stream logs for pod %s: %w", pod.Name, err)
	}
	return &LogStream{
		Pod:       pod.Name,
		Container: container,
		Stream:    stream,
	}, nil
}

// SnapshotWorkloadLogs returns a non-follow snapshot of recent logs.
func (c *Client) SnapshotWorkloadLogs(ctx context.Context, namespace, name string, tailLines int64) (string, string, error) {
	if name == "" {
		return "", "", fmt.Errorf("snapshot logs: workload name is required")
	}
	if namespace == "" {
		namespace = "default"
	}
	if tailLines <= 0 {
		tailLines = 200
	}

	selector := fmt.Sprintf("app=%s", name)
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return "", "", fmt.Errorf("list pods for %s: %w", name, err)
	}
	pod := pickPodForLogs(pods.Items)
	if pod == nil {
		return "", "", fmt.Errorf("no pods found for workload %q", name)
	}

	container := name
	if len(pod.Spec.Containers) > 0 {
		found := false
		for _, ctn := range pod.Spec.Containers {
			if ctn.Name == name {
				found = true
				break
			}
		}
		if !found {
			container = pod.Spec.Containers[0].Name
		}
	}

	opts := &corev1.PodLogOptions{
		Container: container,
		TailLines: &tailLines,
	}
	req := c.clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, opts)
	stream, err := req.Stream(ctx)
	if err != nil {
		return "", pod.Name, fmt.Errorf("read logs for pod %s: %w", pod.Name, err)
	}
	defer stream.Close()
	data, err := io.ReadAll(stream)
	if err != nil {
		return "", pod.Name, err
	}
	return string(data), pod.Name, nil
}

// pickPodForLogs prefers Running ready pods, then any Running, then newest by creation.
func pickPodForLogs(pods []corev1.Pod) *corev1.Pod {
	if len(pods) == 0 {
		return nil
	}
	var best *corev1.Pod
	bestScore := -1
	for i := range pods {
		p := &pods[i]
		score := 0
		switch p.Status.Phase {
		case corev1.PodRunning:
			score = 2
		case corev1.PodPending:
			score = 1
		default:
			score = 0
		}
		if podReady(p) {
			score += 2
		}
		if best == nil || score > bestScore || (score == bestScore && p.CreationTimestamp.After(best.CreationTimestamp.Time)) {
			best = p
			bestScore = score
		}
	}
	return best
}

func podReady(p *corev1.Pod) bool {
	for _, c := range p.Status.Conditions {
		if c.Type == corev1.PodReady && c.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
