package runtime

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPickPodForLogsPrefersReadyRunning(t *testing.T) {
	now := metav1.NewTime(time.Now())
	older := metav1.NewTime(time.Now().Add(-time.Hour))
	pods := []corev1.Pod{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "old-fail", CreationTimestamp: older},
			Status:     corev1.PodStatus{Phase: corev1.PodFailed},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "running", CreationTimestamp: older},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "ready", CreationTimestamp: now},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
				Conditions: []corev1.PodCondition{
					{Type: corev1.PodReady, Status: corev1.ConditionTrue},
				},
			},
		},
	}
	got := pickPodForLogs(pods)
	if got == nil || got.Name != "ready" {
		t.Fatalf("got %#v", got)
	}
}

func TestPickPodForLogsEmpty(t *testing.T) {
	if pickPodForLogs(nil) != nil {
		t.Fatal("expected nil")
	}
}
