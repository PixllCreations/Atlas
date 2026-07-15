package redis

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"

	"github.com/pixll/atlas/dependency"
	"github.com/pixll/atlas/runtime"
)

const (
	// Image is a pinned Redis Alpine release.
	Image = "redis:7.4.2-alpine"
	Port  = int32(6379)
)

// Runtime is the subset of Kubernetes operations the Redis provisioner needs.
type Runtime interface {
	EnsureDeployment(ctx context.Context, opts runtime.DeployOptions) error
	EnsureService(ctx context.Context, opts runtime.ServiceOptions) error
}

// Provisioner provisions a Redis Deployment and ClusterIP Service.
type Provisioner struct {
	runtime Runtime
}

// NewProvisioner returns a Redis dependency provisioner.
func NewProvisioner(rt Runtime) *Provisioner {
	return &Provisioner{runtime: rt}
}

// Provision ensures Redis workloads and returns connection env vars.
func (p *Provisioner) Provision(ctx context.Context, opts dependency.ProvisionOptions) (dependency.ProvisionResult, error) {
	if opts.Name == "" {
		return dependency.ProvisionResult{}, fmt.Errorf("redis: name is required")
	}
	if opts.Namespace == "" {
		return dependency.ProvisionResult{}, fmt.Errorf("redis: namespace is required")
	}

	labels := map[string]string{
		runtime.LabelComponent: ComponentLabel(),
		runtime.LabelDepName:   opts.Name,
		runtime.LabelDepType:   string(opts.Config.Type),
	}

	if err := p.runtime.EnsureDeployment(ctx, runtime.DeployOptions{
		Namespace:   opts.Namespace,
		Name:        opts.Name,
		Image:       Image,
		Port:        Port,
		ProjectID:   opts.ProjectID,
		ProjectName: opts.ProjectName,
		Labels:      labels,
	}); err != nil {
		return dependency.ProvisionResult{}, fmt.Errorf("redis deployment: %w", err)
	}

	if err := p.runtime.EnsureService(ctx, runtime.ServiceOptions{
		Namespace:     opts.Namespace,
		Name:          opts.Name,
		Port:          Port,
		ContainerPort: Port,
		PortName:      "redis",
		ProjectID:     opts.ProjectID,
		ProjectName:   opts.ProjectName,
		Labels:        labels,
	}); err != nil {
		return dependency.ProvisionResult{}, fmt.Errorf("redis service: %w", err)
	}

	return dependency.ProvisionResult{
		Env: []corev1.EnvVar{
			{
				Name:  "REDIS_URL",
				Value: fmt.Sprintf("redis://%s:%d", opts.Name, Port),
			},
		},
	}, nil
}

// ComponentLabel returns the Atlas component label value for dependencies.
func ComponentLabel() string {
	return runtime.ComponentDependency
}
