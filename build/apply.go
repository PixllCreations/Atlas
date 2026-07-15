package build

import (
	"context"
	"fmt"
	"log"

	corev1 "k8s.io/api/core/v1"

	"github.com/pixll/atlas/config"
	"github.com/pixll/atlas/dependency"
	"github.com/pixll/atlas/plan"
	"github.com/pixll/atlas/runtime"
)

// ApplyPlan provisions the namespace, dependencies, and application from a DeploymentPlan.
func (w *Worker) ApplyPlan(ctx context.Context, p plan.DeploymentPlan) error {
	if w.deployer == nil {
		return nil
	}

	if err := w.deployer.EnsureNamespace(ctx, runtime.NamespaceOptions{
		Name:        p.Namespace,
		ProjectID:   p.ProjectID,
		ProjectName: p.ProjectName,
	}); err != nil {
		return fmt.Errorf("ensure namespace: %w", err)
	}

	desiredDeps := make(map[string]struct{}, len(p.Dependencies))
	for _, dep := range p.Dependencies {
		desiredDeps[dep.Name] = struct{}{}
		provisioner, err := w.deps.Get(dep.Type)
		if err != nil {
			return err
		}
		if _, err := provisioner.Provision(ctx, dependency.ProvisionOptions{
			Namespace:   p.Namespace,
			Name:        dep.Name,
			ProjectID:   p.ProjectID,
			ProjectName: p.ProjectName,
			Config:      dep.Config,
		}); err != nil {
			return fmt.Errorf("provision %s: %w", dep.Name, err)
		}
	}

	appLabels := map[string]string{
		runtime.LabelComponent: runtime.ComponentApplication,
	}

	if err := w.deployer.EnsureDeployment(ctx, runtime.DeployOptions{
		Namespace:   p.Namespace,
		Name:        p.Application.Name,
		Image:       p.Application.Image,
		Port:        p.Application.Port,
		Env:         append([]corev1.EnvVar(nil), p.Application.Env...),
		ProjectID:   p.ProjectID,
		ProjectName: p.ProjectName,
		Labels:      appLabels,
	}); err != nil {
		return fmt.Errorf("deploy app: %w", err)
	}

	if err := w.deployer.EnsureService(ctx, runtime.ServiceOptions{
		Namespace:     p.Namespace,
		Name:          p.Application.Name,
		Port:          80,
		ContainerPort: p.Application.Port,
		ProjectID:     p.ProjectID,
		ProjectName:   p.ProjectName,
		Labels:        appLabels,
	}); err != nil {
		return fmt.Errorf("service app: %w", err)
	}

	if p.Host != "" {
		if err := w.deployer.EnsureIngress(ctx, runtime.IngressOptions{
			Namespace:        p.Namespace,
			Name:             p.Application.Name,
			Host:             p.Host,
			Port:             80,
			IngressClassName: w.cfg.IngressClass,
			TLSSecretName:    w.cfg.IngressTLSSecret,
			ProjectID:        p.ProjectID,
			ProjectName:      p.ProjectName,
			Labels:           appLabels,
		}); err != nil {
			return fmt.Errorf("ingress app: %w", err)
		}
	}

	if err := w.pruneDependencies(ctx, p, desiredDeps); err != nil {
		return err
	}
	return nil
}

func (w *Worker) pruneDependencies(ctx context.Context, p plan.DeploymentPlan, desired map[string]struct{}) error {
	deps, err := w.deployer.ListManagedDependencyDeployments(ctx, p.Namespace, p.ProjectID)
	if err != nil {
		return fmt.Errorf("list dependency deployments: %w", err)
	}
	for _, dep := range deps {
		if _, ok := desired[dep.Name]; ok {
			continue
		}
		if err := w.deployer.DeleteDeployment(ctx, p.Namespace, dep.Name); err != nil {
			return fmt.Errorf("delete dependency deployment %s: %w", dep.Name, err)
		}
	}

	svcs, err := w.deployer.ListManagedDependencyServices(ctx, p.Namespace, p.ProjectID)
	if err != nil {
		return fmt.Errorf("list dependency services: %w", err)
	}
	for _, svc := range svcs {
		if _, ok := desired[svc.Name]; ok {
			continue
		}
		if err := w.deployer.DeleteService(ctx, p.Namespace, svc.Name); err != nil {
			return fmt.Errorf("delete dependency service %s: %w", svc.Name, err)
		}
	}
	return nil
}

func (w *Worker) cleanupLegacyResources(ctx context.Context, appName string) {
	if w.deployer == nil || appName == "" {
		return
	}
	ns := w.cfg.Namespace
	if ns == "" {
		ns = "default"
	}
	if err := w.deployer.DeleteIngress(ctx, ns, appName); err != nil {
		log.Printf("legacy teardown ingress %s/%s: %v", ns, appName, err)
	}
	if err := w.deployer.DeleteService(ctx, ns, appName); err != nil {
		log.Printf("legacy teardown service %s/%s: %v", ns, appName, err)
	}
	if err := w.deployer.DeleteDeployment(ctx, ns, appName); err != nil {
		log.Printf("legacy teardown deployment %s/%s: %v", ns, appName, err)
	}
}

// SnapshotFromPlan builds a persistable infrastructure snapshot from a plan.
func SnapshotFromPlan(p plan.DeploymentPlan) map[string]any {
	deps := make([]map[string]any, 0, len(p.Dependencies))
	for _, d := range p.Dependencies {
		endpoint := ""
		switch d.Type {
		case config.DependencyRedis:
			endpoint = fmt.Sprintf("%s:6379", d.Name)
		}
		deps = append(deps, map[string]any{
			"name":     d.Name,
			"type":     string(d.Type),
			"endpoint": endpoint,
		})
	}
	return map[string]any{
		"namespace": p.Namespace,
		"host":      p.Host,
		"app": map[string]any{
			"name": p.Application.Name,
			"port": p.Application.Port,
		},
		"dependencies": deps,
	}
}
