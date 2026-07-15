package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseValid(t *testing.T) {
	cfg, err := Parse([]byte(`
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
	if cfg.Version != 1 {
		t.Fatalf("Version = %d, want 1", cfg.Version)
	}
	if cfg.App.Port != 8080 {
		t.Fatalf("Port = %d, want 8080", cfg.App.Port)
	}
	dep, ok := cfg.Dependencies["redis"]
	if !ok {
		t.Fatal("missing redis dependency")
	}
	if dep.Type != DependencyRedis {
		t.Fatalf("Type = %q, want redis", dep.Type)
	}
}

func TestParseValidNoDependencies(t *testing.T) {
	cfg, err := Parse([]byte(`
version: 1
app:
  port: 80
`))
	if err != nil {
		t.Fatalf("Parse() = %v", err)
	}
	if len(cfg.Dependencies) != 0 {
		t.Fatalf("Dependencies = %v, want empty", cfg.Dependencies)
	}
}

func TestLoadMissing(t *testing.T) {
	dir := t.TempDir()
	_, err := Load(filepath.Join(dir, "atlas.yaml"))
	if !errors.Is(err, ErrMissing) {
		t.Fatalf("Load() = %v, want ErrMissing", err)
	}
	if !strings.Contains(err.Error(), "atlas.yaml") {
		t.Fatalf("error should mention atlas.yaml: %v", err)
	}
}

func TestLoadValid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "atlas.yaml")
	if err := os.WriteFile(path, []byte("version: 1\napp:\n  port: 3000\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load() = %v", err)
	}
	if cfg.App.Port != 3000 {
		t.Fatalf("Port = %d, want 3000", cfg.App.Port)
	}
}

func TestUnsupportedVersion(t *testing.T) {
	_, err := Parse([]byte("version: 2\napp:\n  port: 8080\n"))
	if err == nil || !strings.Contains(err.Error(), "version") {
		t.Fatalf("Parse() = %v, want version error", err)
	}
}

func TestInvalidPort(t *testing.T) {
	_, err := Parse([]byte("version: 1\napp:\n  port: 0\n"))
	if err == nil || !strings.Contains(err.Error(), "port") {
		t.Fatalf("Parse() = %v, want port error", err)
	}
}

func TestUnsupportedDependency(t *testing.T) {
	_, err := Parse([]byte(`
version: 1
app:
  port: 8080
dependencies:
  db:
    type: postgres
`))
	if err == nil || !strings.Contains(err.Error(), "not yet supported") {
		t.Fatalf("Parse() = %v, want unsupported dependency error", err)
	}
}

func TestUnknownDependencyType(t *testing.T) {
	_, err := Parse([]byte(`
version: 1
app:
  port: 8080
dependencies:
  foo:
    type: mongodb
`))
	if err == nil || !strings.Contains(err.Error(), "unsupported type") {
		t.Fatalf("Parse() = %v, want unsupported type error", err)
	}
}

func TestInvalidDependencyName(t *testing.T) {
	_, err := Parse([]byte(`
version: 1
app:
  port: 8080
dependencies:
  Redis_Cache:
    type: redis
`))
	if err == nil || !strings.Contains(err.Error(), "Kubernetes resource name") {
		t.Fatalf("Parse() = %v, want name error", err)
	}
}

func TestReservedDependencyName(t *testing.T) {
	_, err := Parse([]byte(`
version: 1
app:
  port: 8080
dependencies:
  app:
    type: redis
`))
	if err == nil || !strings.Contains(err.Error(), "reserved") {
		t.Fatalf("Parse() = %v, want reserved name error", err)
	}
}

func TestMultipleRedisDependencies(t *testing.T) {
	_, err := Parse([]byte(`
version: 1
app:
  port: 8080
dependencies:
  redis:
    type: redis
  cache:
    type: redis
`))
	if err == nil || !strings.Contains(err.Error(), "only one redis") {
		t.Fatalf("Parse() = %v, want multiple redis error", err)
	}
}
