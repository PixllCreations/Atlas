package dependency

import (
	"context"

	corev1 "k8s.io/api/core/v1"

	"github.com/pixll/atlas/config"
)

// ProvisionOptions configures provisioning of a managed dependency.
type ProvisionOptions struct {
	Namespace   string
	Name        string
	ProjectID   string
	ProjectName string
	Config      config.Dependency
}

// ProvisionResult is returned after a dependency is provisioned.
type ProvisionResult struct {
	Env []corev1.EnvVar
}

// Provisioner creates or updates Kubernetes resources for a dependency type.
type Provisioner interface {
	Provision(ctx context.Context, opts ProvisionOptions) (ProvisionResult, error)
}
