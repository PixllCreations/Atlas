package plan

import (
	"testing"

	"github.com/pixll/atlas/config"
)

func TestBuildWeKnowBall(t *testing.T) {
	cfg, err := config.Parse([]byte(`
version: 1
app:
  port: 8080
dependencies:
  redis:
    type: redis
`))
	if err != nil {
		t.Fatalf("Parse() = %v", err)
	}

	p, err := Build(BuildOptions{
		ProjectID:     "proj-1",
		ProjectName:   "we-know-ball",
		Image:         "localhost:5000/atlas/proj-1:build-1",
		IngressDomain: "edwardscott.dev",
		Config:        cfg,
	})
	if err != nil {
		t.Fatalf("Build() = %v", err)
	}

	if p.Namespace != "atlas-we-know-ball" {
		t.Fatalf("Namespace = %q, want atlas-we-know-ball", p.Namespace)
	}
	if p.Host != "we-know-ball.edwardscott.dev" {
		t.Fatalf("Host = %q, want we-know-ball.edwardscott.dev", p.Host)
	}
	if p.Application.Name != "app" {
		t.Fatalf("Application.Name = %q, want app", p.Application.Name)
	}
	if p.Application.Port != 8080 {
		t.Fatalf("Application.Port = %d, want 8080", p.Application.Port)
	}
	if len(p.Dependencies) != 1 || p.Dependencies[0].Name != "redis" {
		t.Fatalf("Dependencies = %+v, want one redis", p.Dependencies)
	}

	env := map[string]string{}
	for _, e := range p.Application.Env {
		env[e.Name] = e.Value
	}
	if env["PORT"] != "8080" {
		t.Fatalf("PORT = %q, want 8080", env["PORT"])
	}
	if env["REDIS_URL"] != "redis://redis:6379" {
		t.Fatalf("REDIS_URL = %q, want redis://redis:6379", env["REDIS_URL"])
	}

	// Deterministic env order: PORT before REDIS_URL alphabetically... actually PORT < REDIS_URL
	if len(p.Application.Env) < 2 {
		t.Fatalf("expected at least 2 env vars, got %d", len(p.Application.Env))
	}
	if p.Application.Env[0].Name != "PORT" || p.Application.Env[1].Name != "REDIS_URL" {
		t.Fatalf("env order = %v, want PORT then REDIS_URL", p.Application.Env)
	}
}

func TestBuildNamedRedisCache(t *testing.T) {
	cfg, err := config.Parse([]byte(`
version: 1
app:
  port: 8080
dependencies:
  cache:
    type: redis
`))
	if err != nil {
		t.Fatalf("Parse() = %v", err)
	}

	p, err := Build(BuildOptions{
		ProjectID:   "proj-1",
		ProjectName: "demo",
		Image:       "img:tag",
		Config:      cfg,
	})
	if err != nil {
		t.Fatalf("Build() = %v", err)
	}

	found := false
	for _, e := range p.Application.Env {
		if e.Name == "REDIS_URL" {
			found = true
			if e.Value != "redis://cache:6379" {
				t.Fatalf("REDIS_URL = %q, want redis://cache:6379", e.Value)
			}
		}
	}
	if !found {
		t.Fatal("REDIS_URL missing")
	}
}

func TestNamespaceName(t *testing.T) {
	if got := NamespaceName("we-know-ball"); got != "atlas-we-know-ball" {
		t.Fatalf("NamespaceName = %q", got)
	}
}
