package redis

import (
	"context"
	"testing"

	"github.com/pixll/atlas/config"
	"github.com/pixll/atlas/dependency"
	"github.com/pixll/atlas/runtime"
)

type fakeRuntime struct {
	deployments []runtime.DeployOptions
	services    []runtime.ServiceOptions
}

func (f *fakeRuntime) EnsureDeployment(_ context.Context, opts runtime.DeployOptions) error {
	f.deployments = append(f.deployments, opts)
	return nil
}

func (f *fakeRuntime) EnsureService(_ context.Context, opts runtime.ServiceOptions) error {
	f.services = append(f.services, opts)
	return nil
}

func TestProvisionerRedis(t *testing.T) {
	rt := &fakeRuntime{}
	p := NewProvisioner(rt)

	res, err := p.Provision(context.Background(), dependency.ProvisionOptions{
		Namespace:   "atlas-demo",
		Name:        "redis",
		ProjectID:   "pid",
		ProjectName: "demo",
		Config:      config.Dependency{Type: config.DependencyRedis},
	})
	if err != nil {
		t.Fatalf("Provision() = %v", err)
	}

	if len(rt.deployments) != 1 {
		t.Fatalf("deployments = %d, want 1", len(rt.deployments))
	}
	d := rt.deployments[0]
	if d.Name != "redis" || d.Image != Image || d.Port != Port {
		t.Fatalf("deployment = %+v", d)
	}
	if d.Labels[runtime.LabelComponent] != runtime.ComponentDependency {
		t.Fatalf("component label = %q", d.Labels[runtime.LabelComponent])
	}

	if len(rt.services) != 1 {
		t.Fatalf("services = %d, want 1", len(rt.services))
	}
	s := rt.services[0]
	if s.Port != Port || s.ContainerPort != Port {
		t.Fatalf("service ports = %+v", s)
	}
	if s.PortName != "redis" {
		t.Fatalf("PortName = %q", s.PortName)
	}

	if len(res.Env) != 1 || res.Env[0].Name != "REDIS_URL" || res.Env[0].Value != "redis://redis:6379" {
		t.Fatalf("Env = %+v", res.Env)
	}
}

func TestProvisionerNamedCache(t *testing.T) {
	rt := &fakeRuntime{}
	p := NewProvisioner(rt)

	res, err := p.Provision(context.Background(), dependency.ProvisionOptions{
		Namespace: "atlas-demo",
		Name:      "cache",
		Config:    config.Dependency{Type: config.DependencyRedis},
	})
	if err != nil {
		t.Fatalf("Provision() = %v", err)
	}
	if res.Env[0].Value != "redis://cache:6379" {
		t.Fatalf("REDIS_URL = %q", res.Env[0].Value)
	}
}
