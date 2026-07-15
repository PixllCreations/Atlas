package runtime

import (
	"context"
	"fmt"
	"io"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TailBuildJobLogs returns concatenated logs from the build Job's pods (init + containers).
func (c *Client) TailBuildJobLogs(ctx context.Context, namespace, buildID string) (string, error) {
	if buildID == "" {
		return "", fmt.Errorf("tail build logs: build id is required")
	}
	if namespace == "" {
		namespace = "default"
	}

	selector := "atlas.build/id=" + buildID
	pods, err := c.clientset.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return "", fmt.Errorf("list build pods: %w", err)
	}
	if len(pods.Items) == 0 {
		return "", nil
	}

	var b strings.Builder
	for _, pod := range pods.Items {
		for _, ctn := range append(pod.Spec.InitContainers, pod.Spec.Containers...) {
			opts := &corev1.PodLogOptions{
				Container: ctn.Name,
			}
			req := c.clientset.CoreV1().Pods(namespace).GetLogs(pod.Name, opts)
			stream, err := req.Stream(ctx)
			if err != nil {
				// Pod/container may not be ready yet.
				continue
			}
			data, readErr := io.ReadAll(stream)
			_ = stream.Close()
			if readErr != nil {
				continue
			}
			if len(data) == 0 {
				continue
			}
			if b.Len() > 0 {
				b.WriteByte('\n')
			}
			b.WriteString("--- ")
			b.WriteString(ctn.Name)
			b.WriteString(" ---\n")
			b.Write(data)
		}
	}
	return b.String(), nil
}
