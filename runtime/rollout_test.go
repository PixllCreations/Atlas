package runtime

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	apps "k8s.io/api/apps/v1"
	core "k8s.io/api/core/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func rolloutFixture(t *testing.T) (*Client, *apps.Deployment, *apps.ReplicaSet, *core.Pod, RolloutOptions) {
	t.Helper()
	d := desiredDeployment(DeployOptions{Name: "app", Image: "image:new", Revision: "build-new"})
	d.Namespace = "atlas-demo"
	d.UID = "deployment"
	d.Generation = 2
	d.Status = apps.DeploymentStatus{ObservedGeneration: 2, Replicas: 1, UpdatedReplicas: 1, AvailableReplicas: 1}
	yes := true
	owner := func(uid types.UID) []meta.OwnerReference { return []meta.OwnerReference{{UID: uid, Controller: &yes}} }
	r := &apps.ReplicaSet{ObjectMeta: meta.ObjectMeta{Name: "app-new", Namespace: d.Namespace, UID: "new-rs", OwnerReferences: owner(d.UID), Generation: 1}, Spec: apps.ReplicaSetSpec{Template: d.Spec.Template}, Status: apps.ReplicaSetStatus{ObservedGeneration: 1, AvailableReplicas: 1}}
	p := &core.Pod{ObjectMeta: meta.ObjectMeta{Name: "app-new-pod", Namespace: d.Namespace, UID: "new-pod", OwnerReferences: owner(r.UID)}, Spec: d.Spec.Template.Spec, Status: core.PodStatus{Phase: core.PodRunning, Conditions: []core.PodCondition{{Type: core.PodReady, Status: core.ConditionTrue}}, ContainerStatuses: []core.ContainerStatus{{Name: "app", Ready: true, State: core.ContainerState{Running: &core.ContainerStateRunning{}}}}}}
	cs := fake.NewClientset(d, r, p)
	c := &Client{clientset: cs, rolloutLog: func(context.Context, string, string, *core.PodLogOptions) (string, error) {
		return "", errors.New("logs unavailable")
	}}
	return c, d, r, p, RolloutOptions{Namespace: d.Namespace, Name: d.Name, Revision: "build-new", Image: "image:new", Timeout: 30 * time.Millisecond}
}

func saveFixture(t *testing.T, c *Client, d *apps.Deployment, r *apps.ReplicaSet, p *core.Pod) {
	t.Helper()
	if _, err := c.clientset.AppsV1().Deployments(d.Namespace).Update(context.Background(), d, meta.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.clientset.AppsV1().ReplicaSets(d.Namespace).Update(context.Background(), r, meta.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.clientset.CoreV1().Pods(d.Namespace).Update(context.Background(), p, meta.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestRolloutSuccess(t *testing.T) {
	c, _, _, _, opts := rolloutFixture(t)
	if err := c.waitForRollout(context.Background(), opts, time.Millisecond); err != nil {
		t.Fatal(err)
	}
}

func TestRolloutFailures(t *testing.T) {
	for _, tc := range []struct {
		name, waiting string
		exit          bool
	}{
		{name: "exit code 1", exit: true}, {name: "crash loop", waiting: "CrashLoopBackOff"},
		{name: "image pull", waiting: "ImagePullBackOff"}, {name: "image pull error", waiting: "ErrImagePull"},
		{name: "config error", waiting: "CreateContainerConfigError"}, {name: "timeout"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, d, r, p, opts := rolloutFixture(t)
			d.Status.Replicas = 2 // an old healthy replica must never prove this rollout ready
			r.Status.AvailableReplicas = 0
			p.Status.Conditions = nil
			s := &p.Status.ContainerStatuses[0]
			s.Ready = false
			s.RestartCount = 6
			s.State = core.ContainerState{}
			if tc.exit {
				s.State.Terminated = &core.ContainerStateTerminated{Reason: "Error", ExitCode: 1}
			}
			if tc.waiting != "" {
				s.State.Waiting = &core.ContainerStateWaiting{Reason: tc.waiting}
				s.LastTerminationState.Terminated = &core.ContainerStateTerminated{Reason: "Error", ExitCode: 1}
			}
			saveFixture(t, c, d, r, p)
			err := c.waitForRollout(context.Background(), opts, time.Millisecond)
			if err == nil {
				t.Fatal("unready new revision was successful")
			}
			for _, want := range []string{"Namespace: atlas-demo", "Workload: app", "Pod: app-new-pod", "Container: app", "Ready: false", "Restarts: 6", "logs unavailable"} {
				if !strings.Contains(err.Error(), want) {
					t.Fatalf("missing %q: %s", want, err)
				}
			}
			if tc.waiting != "" && !strings.Contains(err.Error(), tc.waiting) {
				t.Fatal(err)
			}
			if tc.exit && !strings.Contains(err.Error(), "Exit code: 1") {
				t.Fatal(err)
			}
			if tc.name == "timeout" && !strings.Contains(err.Error(), "timed out") {
				t.Fatal(err)
			}
		})
	}
}

func TestOldRevisionServingNewFails(t *testing.T) {
	c, d, r, p, opts := rolloutFixture(t)
	r.Status.AvailableReplicas = 0
	p.Status.Conditions = nil
	p.Status.ContainerStatuses[0].State = core.ContainerState{Waiting: &core.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}
	old := r.DeepCopy()
	old.Name = "app-old"
	old.UID = "old-rs"
	old.Spec.Template.Annotations = map[string]string{RevisionAnnotation: "old-build"}
	old.Status.AvailableReplicas = 1
	_, err := c.clientset.AppsV1().ReplicaSets(d.Namespace).Create(context.Background(), old, meta.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	oldPod := p.DeepCopy()
	oldPod.Name = "old-pod"
	oldPod.UID = "old-pod"
	oldPod.OwnerReferences[0].UID = old.UID
	oldPod.Status.Conditions = []core.PodCondition{{Type: core.PodReady, Status: core.ConditionTrue}}
	oldPod.Status.ContainerStatuses = nil
	c.clientset.CoreV1().Pods(d.Namespace).Create(context.Background(), oldPod, meta.CreateOptions{})
	saveFixture(t, c, d, r, p)
	err = c.waitForRollout(context.Background(), opts, time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "Previous revision remains available") || strings.Contains(err.Error(), "Pod: old-pod") {
		t.Fatalf("incorrect revision diagnostics: %v", err)
	}
}

func TestPreviousLogsUnavailableFallsBackAndRedacts(t *testing.T) {
	c, d, r, p, opts := rolloutFixture(t)
	p.Spec.Containers[0].Env = []core.EnvVar{{Name: "VAPID_PRIVATE_KEY", Value: `"sensitive-runtime-value"`}}
	opts.SensitiveValues = []string{`"sensitive-runtime-value"`}
	s := &p.Status.ContainerStatuses[0]
	s.RestartCount = 6
	s.State = core.ContainerState{Waiting: &core.ContainerStateWaiting{Reason: "CrashLoopBackOff", Message: `bad value "sensitive-runtime-value"`}}
	c.clientset.CoreV1().Events(d.Namespace).Create(context.Background(), &core.Event{ObjectMeta: meta.ObjectMeta{Name: "backoff", Namespace: d.Namespace}, InvolvedObject: core.ObjectReference{UID: p.UID}, Reason: "BackOff", Message: `Back-off restarting failed container`}, meta.CreateOptions{})
	saveFixture(t, c, d, r, p)
	var calls []bool
	c.rolloutLog = func(_ context.Context, _, _ string, o *core.PodLogOptions) (string, error) {
		calls = append(calls, o.Previous)
		if o.Previous {
			return "", errors.New("previous unavailable: must not echo this error")
		}
		return "startup failed for \"sensitive-runtime-value\"\npassword=another-password\n", nil
	}
	err := c.waitForRollout(context.Background(), opts, time.Millisecond)
	if err == nil {
		t.Fatal("expected failure")
	}
	text := err.Error()
	for _, secret := range []string{"sensitive-runtime-value", "another-password", "unexpected-token", "must not echo"} {
		if strings.Contains(text, secret) {
			t.Fatalf("secret or raw error leaked: %s", text)
		}
	}
	if !strings.Contains(text, "Back-off restarting failed container") {
		t.Fatal(text)
	}
	if len(calls) != 2 || !calls[0] || calls[1] {
		t.Fatalf("fallback calls: %v", calls)
	}
	if !strings.Contains(text, "Event:") || !strings.Contains(text, "previous=false") {
		t.Fatal(text)
	}
}

func TestSupersededOrUnobservedRevisionCannotSucceed(t *testing.T) {
	for _, superseded := range []bool{false, true} {
		c, d, r, p, opts := rolloutFixture(t)
		if superseded {
			d.Spec.Template.Annotations[RevisionAnnotation] = "other"
		} else {
			d.Status.ObservedGeneration = 1
		}
		saveFixture(t, c, d, r, p)
		if err := c.waitForRollout(context.Background(), opts, time.Millisecond); err == nil {
			t.Fatal("stale generation/revision accepted")
		}
	}
}

func TestMissingBothLogsStillReportsEvents(t *testing.T) {
	c, d, r, p, opts := rolloutFixture(t)
	p.Status.ContainerStatuses[0].State = core.ContainerState{Waiting: &core.ContainerStateWaiting{Reason: "CrashLoopBackOff"}}
	p.Status.ContainerStatuses[0].RestartCount = 6
	p.Status.ContainerStatuses[0].Ready = false
	p.Status.Conditions = nil
	saveFixture(t, c, d, r, p)
	_, err := c.clientset.CoreV1().Events(d.Namespace).Create(context.Background(), &core.Event{ObjectMeta: meta.ObjectMeta{Name: "backoff", Namespace: d.Namespace}, InvolvedObject: core.ObjectReference{UID: p.UID}, Reason: "BackOff", Message: "Back-off restarting failed container"}, meta.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	err = c.waitForRollout(context.Background(), opts, time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "Container logs unavailable") || !strings.Contains(err.Error(), "Back-off restarting failed container") {
		t.Fatalf("missing fallback details: %v", err)
	}
}

func TestReadyButNotYetAvailableTimesOut(t *testing.T) {
	c, d, r, p, opts := rolloutFixture(t)
	r.Status.AvailableReplicas = 0
	saveFixture(t, c, d, r, p)
	if err := c.waitForRollout(context.Background(), opts, time.Millisecond); err == nil {
		t.Fatal("ready pod bypassed availability/stability requirement")
	}
}

func TestExternalEnvironmentWithholdsLogTail(t *testing.T) {
	c, d, r, p, opts := rolloutFixture(t)
	p.Spec.Containers[0].EnvFrom = []core.EnvFromSource{{SecretRef: &core.SecretEnvSource{LocalObjectReference: core.LocalObjectReference{Name: "credentials"}}}}
	p.Status.ContainerStatuses[0].State = core.ContainerState{Waiting: &core.ContainerStateWaiting{Reason: "CreateContainerConfigError"}}
	saveFixture(t, c, d, r, p)
	c.rolloutLog = func(context.Context, string, string, *core.PodLogOptions) (string, error) {
		t.Fatal("must not fetch logs containing unknown externally sourced values")
		return "", nil
	}
	if err := c.waitForRollout(context.Background(), opts, time.Millisecond); err == nil || !strings.Contains(err.Error(), "logs withheld") {
		t.Fatal(err)
	}
}

func TestRolloutWaitsForControllerObservation(t *testing.T) {
	c, d, _, _, opts := rolloutFixture(t)
	calls := 0
	c.clientset.(*fake.Clientset).PrependReactor("get", "deployments", func(action ktesting.Action) (bool, kruntime.Object, error) {
		calls++
		observed := d.DeepCopy()
		if calls == 1 {
			observed.Status.ObservedGeneration = 1
		}
		return true, observed, nil
	})
	if err := c.waitForRollout(context.Background(), opts, time.Millisecond); err != nil {
		t.Fatal(err)
	}
	if calls < 2 {
		t.Fatal("returned before controller observed new generation")
	}
}
