package runtime

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/pixll/atlas/internal/redact"
	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

const RevisionAnnotation = "atlas.build/revision"
const DefaultRolloutTimeout = 5 * time.Minute

type RolloutOptions struct {
	Namespace, Name, Revision, Image string
	Timeout                          time.Duration
	SensitiveValues                  []string
}

// WaitForRollout follows only ReplicaSets and pods owned by this deployment and
// stamped with this build's revision. Deployment-wide availability alone is insufficient.
func (c *Client) WaitForRollout(ctx context.Context, opts RolloutOptions) error {
	return c.waitForRollout(ctx, opts, time.Second)
}

func (c *Client) waitForRollout(parent context.Context, opts RolloutOptions, interval time.Duration) error {
	if opts.Revision == "" || opts.Image == "" {
		return fmt.Errorf("rollout requires an intended revision and image")
	}
	if opts.Timeout <= 0 {
		opts.Timeout = DefaultRolloutTimeout
	}
	ctx, cancel := context.WithTimeout(parent, opts.Timeout)
	defer cancel()
	var dep *apps.Deployment
	var pods []core.Pod
	var sets []apps.ReplicaSet
	finish := func(reason string) error {
		// A separate bounded context permits diagnostics even after the rollout deadline.
		diagCtx, stop := context.WithTimeout(context.WithoutCancel(parent), 10*time.Second)
		defer stop()
		return fmt.Errorf("%s", c.rolloutDiagnostics(diagCtx, opts, reason, dep, sets, pods))
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		var err error
		dep, err = c.clientset.AppsV1().Deployments(opts.Namespace).Get(ctx, opts.Name, meta.GetOptions{})
		if err != nil {
			if ctx.Err() != nil {
				return finish("Rollout timed out or was cancelled.")
			}
			return finish("Unable to read Deployment status.")
		}
		if dep.Spec.Template.Annotations[RevisionAnnotation] != opts.Revision || !hasImage(dep.Spec.Template.Spec.Containers, opts.Image) {
			return finish("The intended revision was superseded by another deployment.")
		}
		rs, err := c.clientset.AppsV1().ReplicaSets(opts.Namespace).List(ctx, meta.ListOptions{})
		if err != nil {
			return finish("Unable to read ReplicaSet status.")
		}
		sets = nil
		ids := map[types.UID]bool{}
		for _, r := range rs.Items {
			if controlledBy(r.OwnerReferences, dep.UID) {
				sets = append(sets, r)
				if r.Spec.Template.Annotations[RevisionAnnotation] == opts.Revision && hasImage(r.Spec.Template.Spec.Containers, opts.Image) {
					ids[r.UID] = true
				}
			}
		}
		list, err := c.clientset.CoreV1().Pods(opts.Namespace).List(ctx, meta.ListOptions{})
		if err != nil {
			return finish("Unable to read pod status.")
		}
		pods = nil
		for _, p := range list.Items {
			for id := range ids {
				if controlledBy(p.OwnerReferences, id) {
					pods = append(pods, p)
					break
				}
			}
		}
		for _, p := range pods {
			if p.DeletionTimestamp != nil {
				continue
			}
			if p.Status.Phase == core.PodFailed {
				return finish("New application pod failed.")
			}
			for _, s := range append(append([]core.ContainerStatus{}, p.Status.InitContainerStatuses...), p.Status.ContainerStatuses...) {
				if s.State.Terminated != nil && s.State.Terminated.ExitCode != 0 {
					return finish("New application container exited unsuccessfully.")
				}
				if s.State.Waiting != nil {
					switch s.State.Waiting.Reason {
					case "CrashLoopBackOff", "ImagePullBackOff", "ErrImagePull", "CreateContainerConfigError", "CreateContainerError", "InvalidImageName", "RunContainerError":
						return finish("New application container could not start.")
					}
				}
			}
		}
		desired := int32(1)
		if dep.Spec.Replicas != nil {
			desired = *dep.Spec.Replicas
		}
		var available int32
		for _, r := range sets {
			if ids[r.UID] && r.Status.ObservedGeneration >= r.Generation {
				available += r.Status.AvailableReplicas
			}
		}
		ready := int32(0)
		for _, p := range pods {
			if p.DeletionTimestamp == nil && podReady(&p) {
				ready++
			}
		}
		if desired > 0 && dep.Status.ObservedGeneration >= dep.Generation && dep.Status.UpdatedReplicas == desired && dep.Status.Replicas == desired && dep.Status.AvailableReplicas >= desired && available >= desired && ready >= desired {
			return nil
		}
		for _, cond := range dep.Status.Conditions {
			if dep.Status.ObservedGeneration >= dep.Generation && cond.Type == apps.DeploymentProgressing && cond.Status == core.ConditionFalse && cond.Reason == "ProgressDeadlineExceeded" {
				return finish("Deployment exceeded its progress deadline.")
			}
		}
		select {
		case <-ctx.Done():
			return finish("Rollout timed out or was cancelled.")
		case <-ticker.C:
		}
	}
}

func controlledBy(refs []meta.OwnerReference, uid types.UID) bool {
	for _, r := range refs {
		if r.UID == uid && r.Controller != nil && *r.Controller {
			return true
		}
	}
	return false
}
func hasImage(containers []core.Container, image string) bool {
	for _, c := range containers {
		if c.Image == image {
			return true
		}
	}
	return false
}

func (c *Client) rolloutDiagnostics(ctx context.Context, opts RolloutOptions, reason string, dep *apps.Deployment, sets []apps.ReplicaSet, pods []core.Pod) string {
	var b strings.Builder
	fmt.Fprintf(&b, "New application revision did not become ready.\nNamespace: %s\nWorkload: %s\n%s\n", opts.Namespace, opts.Name, reason)
	for _, rs := range sets {
		if rs.Spec.Template.Annotations[RevisionAnnotation] != opts.Revision && rs.Status.AvailableReplicas > 0 {
			b.WriteString("Previous revision remains available and may still be serving traffic.\n")
			break
		}
	}
	values := append([]string{}, opts.SensitiveValues...)
	if dep != nil {
		for _, c := range dep.Spec.Template.Spec.Containers {
			for _, e := range c.Env {
				values = append(values, e.Value)
			}
		}
	}
	ids := map[types.UID]bool{}
	if dep != nil {
		ids[dep.UID] = true
	}
	for _, rs := range sets {
		if rs.Spec.Template.Annotations[RevisionAnnotation] == opts.Revision {
			ids[rs.UID] = true
		}
	}
	for i, p := range pods {
		if i >= 5 {
			b.WriteString("Additional pods omitted.\n")
			break
		}
		ids[p.UID] = true
		fmt.Fprintf(&b, "Pod: %s\nReadiness: %t\n", p.Name, podReady(&p))
		for _, cond := range p.Status.Conditions {
			if cond.Status == core.ConditionFalse {
				fmt.Fprintf(&b, "Pod condition: %s %s %s\n", cond.Type, cond.Reason, cond.Message)
			}
		}
		containers := append(append([]core.Container{}, p.Spec.InitContainers...), p.Spec.Containers...)
		suppressLogs := false
		for _, ctn := range containers {
			if len(ctn.EnvFrom) > 0 {
				suppressLogs = true
			}
			for _, env := range ctn.Env {
				values = append(values, env.Value)
				if env.ValueFrom != nil {
					suppressLogs = true
				}
			}
		}
		for _, s := range append(append([]core.ContainerStatus{}, p.Status.InitContainerStatuses...), p.Status.ContainerStatuses...) {
			fmt.Fprintf(&b, "Container: %s\nReady: %t\nRestarts: %d\n", s.Name, s.Ready, s.RestartCount)
			if s.State.Waiting != nil {
				fmt.Fprintf(&b, "State: Waiting\nReason: %s\n%s\n", s.State.Waiting.Reason, s.State.Waiting.Message)
			}
			terminated := s.State.Terminated
			if terminated == nil {
				terminated = s.LastTerminationState.Terminated
			}
			if terminated != nil {
				label := "State"
				if s.State.Terminated == nil {
					label = "Previous state"
				}
				fmt.Fprintf(&b, "%s: Terminated\nReason: %s\nExit code: %d\n", label, terminated.Reason, terminated.ExitCode)
			}
			if s.State.Running != nil {
				b.WriteString("State: Running\n")
			}
			if suppressLogs {
				b.WriteString("Container logs withheld because this pod uses externally sourced environment values.\n")
				continue
			}
			logged := false
			for _, previous := range []bool{true, false} {
				if previous && s.RestartCount == 0 {
					continue
				}
				lines, limit := int64(20), int64(4096)
				data, err := c.readRolloutLog(ctx, opts.Namespace, p.Name, &core.PodLogOptions{Container: s.Name, Previous: previous, TailLines: &lines, LimitBytes: &limit})
				if err == nil && len(data) > 0 {
					fmt.Fprintf(&b, "Container logs (previous=%t):\n%s\n", previous, data)
					logged = true
					break
				}
			}
			if !logged {
				b.WriteString("Container logs unavailable; use the pod status and events below.\n")
			}
		}
	}
	if len(pods) == 0 {
		b.WriteString("No pods for the intended revision observed.\n")
	}
	events, err := c.clientset.CoreV1().Events(opts.Namespace).List(ctx, meta.ListOptions{})
	if err == nil {
		sort.SliceStable(events.Items, func(i, j int) bool {
			return events.Items[i].LastTimestamp.Time.Before(events.Items[j].LastTimestamp.Time)
		})
		count := 0
		// Only events referring to the exact Deployment, new ReplicaSet, or new pods.
		for i := len(events.Items) - 1; i >= 0 && count < 8; i-- {
			e := events.Items[i]
			if ids[e.InvolvedObject.UID] {
				fmt.Fprintf(&b, "Event: %s: %s\n", e.Reason, e.Message)
				count++
			}
		}
		if count == 0 {
			b.WriteString("No relevant Kubernetes events available.\n")
		}
	} else {
		b.WriteString("Kubernetes events unavailable.\n")
	}
	return redact.Text(b.String(), values...)
}

func (c *Client) readRolloutLog(ctx context.Context, namespace, pod string, opts *core.PodLogOptions) (string, error) {
	if c.rolloutLog != nil {
		return c.rolloutLog(ctx, namespace, pod, opts)
	}
	stream, err := c.clientset.CoreV1().Pods(namespace).GetLogs(pod, opts).Stream(ctx)
	if err != nil {
		return "", err
	}
	defer stream.Close()
	data, err := io.ReadAll(io.LimitReader(stream, 4096))
	return string(data), err
}
