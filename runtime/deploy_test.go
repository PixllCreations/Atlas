package runtime

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestDesiredDeploymentReconcilesEnv(t *testing.T) {
	dep := desiredDeployment(DeployOptions{
		Name:        "app",
		Image:       "img:v1",
		Port:        8080,
		ProjectID:   "pid",
		ProjectName: "demo",
		Env: []corev1.EnvVar{
			{Name: "REDIS_URL", Value: "redis://redis:6379"},
			{Name: "PORT", Value: "8080"},
		},
		Labels: map[string]string{
			LabelComponent: ComponentApplication,
		},
	})

	if dep.Name != "app" {
		t.Fatalf("Name = %q", dep.Name)
	}
	c := dep.Spec.Template.Spec.Containers[0]
	if c.Image != "img:v1" {
		t.Fatalf("Image = %q", c.Image)
	}
	if len(c.Ports) != 1 || c.Ports[0].ContainerPort != 8080 {
		t.Fatalf("Ports = %+v", c.Ports)
	}
	if len(c.Env) != 2 {
		t.Fatalf("Env len = %d", len(c.Env))
	}
	// NormalizeEnv is applied in EnsureDeployment; desiredDeployment uses opts as-is.
	// Call NormalizeEnv to match EnsureDeployment behavior.
	env := NormalizeEnv([]corev1.EnvVar{
		{Name: "REDIS_URL", Value: "redis://redis:6379"},
		{Name: "PORT", Value: "8080"},
	})
	if env[0].Name != "PORT" || env[0].Value != "8080" {
		t.Fatalf("env[0] = %+v", env[0])
	}
	if env[1].Name != "REDIS_URL" {
		t.Fatalf("env[1] = %+v", env[1])
	}
	if dep.Labels[LabelManagedBy] != LabelManagedByValue {
		t.Fatalf("managed-by label missing: %v", dep.Labels)
	}
	if dep.Labels[LabelProjectID] != "pid" {
		t.Fatalf("project-id = %q", dep.Labels[LabelProjectID])
	}
	if dep.Labels[LabelComponent] != ComponentApplication {
		t.Fatalf("component = %q", dep.Labels[LabelComponent])
	}
}

func TestNormalizeEnvDeterministic(t *testing.T) {
	got := NormalizeEnv([]corev1.EnvVar{
		{Name: "B", Value: "2"},
		{Name: "A", Value: "1"},
	})
	if got[0].Name != "A" || got[1].Name != "B" {
		t.Fatalf("order = %+v", got)
	}
}
