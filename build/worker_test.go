package build

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/pixll/atlas/app"
	"github.com/pixll/atlas/config"
	"github.com/pixll/atlas/plan"
	"github.com/pixll/atlas/runtime"
)

const testAtlasYAML = "version: 1\napp:\n  port: 8080\n"

type fakeBuildStore struct {
	builds    map[string]Build
	repos     map[string]app.Repo
	apps      map[string]app.App
	snapshots map[string][]byte
}

func newFakeBuildStore(builds ...Build) *fakeBuildStore {
	m := make(map[string]Build, len(builds))
	for _, b := range builds {
		m[b.ID] = b
	}
	return &fakeBuildStore{builds: m, snapshots: make(map[string][]byte)}
}

func (f *fakeBuildStore) GetApp(_ context.Context, id string) (app.App, error) {
	a, ok := f.apps[id]
	if !ok {
		return app.App{}, errors.New("app not found")
	}
	return a, nil
}

func (f *fakeBuildStore) GetRepo(_ context.Context, appID string) (app.Repo, error) {
	repo, ok := f.repos[appID]
	if !ok {
		return app.Repo{}, errors.New("repo not found")
	}
	return repo, nil
}

func (f *fakeBuildStore) GetBuild(_ context.Context, id string) (Build, error) {
	b, ok := f.builds[id]
	if !ok {
		return Build{}, errors.New("build not found")
	}
	return b, nil
}

func (f *fakeBuildStore) UpdateBuildStatus(_ context.Context, id string, status Status) (Build, error) {
	b, ok := f.builds[id]
	if !ok {
		return Build{}, errors.New("build not found")
	}
	b.Status = status
	b.UpdatedAt = time.Now()
	f.builds[id] = b
	return b, nil
}

func (f *fakeBuildStore) UpdateBuildImage(_ context.Context, id string, image string) (Build, error) {
	b, ok := f.builds[id]
	if !ok {
		return Build{}, errors.New("build not found")
	}
	b.Image = image
	b.UpdatedAt = time.Now()
	f.builds[id] = b
	return b, nil
}

func (f *fakeBuildStore) UpdateBuildPhase(_ context.Context, id string, phase Phase) (Build, error) {
	b, ok := f.builds[id]
	if !ok {
		return Build{}, errors.New("build not found")
	}
	b.Phase = phase
	b.UpdatedAt = time.Now()
	f.builds[id] = b
	return b, nil
}

func (f *fakeBuildStore) AppendBuildLog(_ context.Context, id string, chunk string) error {
	b, ok := f.builds[id]
	if !ok {
		return errors.New("build not found")
	}
	b.Log += chunk
	b.UpdatedAt = time.Now()
	f.builds[id] = b
	return nil
}

func (f *fakeBuildStore) UpdateAppDeploymentSnapshot(_ context.Context, id string, snapshot []byte) error {
	if f.apps != nil {
		if _, ok := f.apps[id]; !ok {
			return errors.New("app not found")
		}
	}
	f.snapshots[id] = snapshot
	return nil
}

func writeAtlasYAMLClone(t *testing.T, yaml string) func(context.Context, string, string, string) error {
	t.Helper()
	if yaml == "" {
		yaml = testAtlasYAML
	}
	return func(_ context.Context, _, _, dest string) error {
		if err := os.MkdirAll(dest, 0o755); err != nil {
			return err
		}
		return os.WriteFile(filepath.Join(dest, "atlas.yaml"), []byte(yaml), 0o644)
	}
}

func TestWorker_ProcessPendingBuild(t *testing.T) {
	now := time.Now()
	store := newFakeBuildStore(Build{
		ID:        "build-1",
		AppID:     "app-1",
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	})
	store.repos = map[string]app.Repo{
		"app-1": {URL: "https://github.com/user/repo", Branch: "main"},
	}
	store.apps = map[string]app.App{
		"app-1": {ID: "app-1", Name: "demo"},
	}
	worker := NewWorker(store, WorkerConfig{Registry: "localhost:5000"}, nil, nil)
	worker.clone = writeAtlasYAMLClone(t, "")
	worker.buildImage = func(context.Context, string, string) error { return nil }
	worker.pushImage = func(context.Context, string, string) error { return nil }

	if err := worker.Process(context.Background(), "build-1"); err != nil {
		t.Fatalf("Process() = %v, want nil", err)
	}

	got := store.builds["build-1"]
	if got.Status != StatusSucceeded {
		t.Fatalf("status = %q, want %q", got.Status, StatusSucceeded)
	}
	if got.Image != "localhost:5000/atlas/app-1:build-1" {
		t.Fatalf("image = %q, want %q", got.Image, "localhost:5000/atlas/app-1:build-1")
	}
	if store.snapshots["app-1"] == nil {
		t.Fatal("expected deployment snapshot")
	}
}

type fakeDeployer struct {
	buildJobOpts     runtime.BuildJobOptions
	waitNS           string
	waitBuildID      string
	namespaces       []runtime.NamespaceOptions
	deployments      []runtime.DeployOptions
	services         []runtime.ServiceOptions
	ingresses        []runtime.IngressOptions
	deletedDeploys   []string
	deletedServices  []string
	deletedIngresses []string
	deletedNS        []string
	depDeploys       []appsv1.Deployment
	depServices      []corev1.Service
}

func (f *fakeDeployer) EnsureBuildJob(_ context.Context, opts runtime.BuildJobOptions) error {
	f.buildJobOpts = opts
	return nil
}

func (f *fakeDeployer) WaitForBuildJob(_ context.Context, namespace, buildID string, _ func(string)) error {
	f.waitNS = namespace
	f.waitBuildID = buildID
	return nil
}

func (f *fakeDeployer) TailBuildJobLogs(context.Context, string, string) (string, error) {
	return "", nil
}

func (f *fakeDeployer) EnsureNamespace(_ context.Context, opts runtime.NamespaceOptions) error {
	f.namespaces = append(f.namespaces, opts)
	return nil
}

func (f *fakeDeployer) DeleteNamespace(_ context.Context, name string) error {
	f.deletedNS = append(f.deletedNS, name)
	return nil
}

func (f *fakeDeployer) EnsureDeployment(_ context.Context, opts runtime.DeployOptions) error {
	f.deployments = append(f.deployments, opts)
	return nil
}
func (f *fakeDeployer) EnsureService(_ context.Context, opts runtime.ServiceOptions) error {
	f.services = append(f.services, opts)
	return nil
}
func (f *fakeDeployer) EnsureIngress(_ context.Context, opts runtime.IngressOptions) error {
	f.ingresses = append(f.ingresses, opts)
	return nil
}
func (f *fakeDeployer) DeleteDeployment(_ context.Context, ns, name string) error {
	f.deletedDeploys = append(f.deletedDeploys, ns+"/"+name)
	return nil
}
func (f *fakeDeployer) DeleteService(_ context.Context, ns, name string) error {
	f.deletedServices = append(f.deletedServices, ns+"/"+name)
	return nil
}
func (f *fakeDeployer) DeleteIngress(_ context.Context, ns, name string) error {
	f.deletedIngresses = append(f.deletedIngresses, ns+"/"+name)
	return nil
}

func (f *fakeDeployer) ListManagedDependencyDeployments(context.Context, string, string) ([]appsv1.Deployment, error) {
	return f.depDeploys, nil
}

func (f *fakeDeployer) ListManagedDependencyServices(context.Context, string, string) ([]corev1.Service, error) {
	return f.depServices, nil
}

func TestWorker_ProcessPendingBuildJobPath(t *testing.T) {
	now := time.Now()
	store := newFakeBuildStore(Build{
		ID:        "build-1",
		AppID:     "app-1",
		Status:    StatusPending,
		CreatedAt: now,
		UpdatedAt: now,
	})
	store.repos = map[string]app.Repo{
		"app-1": {URL: "https://github.com/user/repo", Branch: "main"},
	}
	store.apps = map[string]app.App{
		"app-1": {ID: "app-1", Name: "portfolio"},
	}

	deployer := &fakeDeployer{}
	worker := NewWorker(store, WorkerConfig{
		Registry:           "localhost:5000",
		Namespace:          "default",
		IngressDomain:      "edwardscott.dev",
		RegistrySecretName: "registry-creds",
		InsecureRegistry:   true,
	}, deployer, nil)
	worker.clone = writeAtlasYAMLClone(t, testAtlasYAML+`
dependencies:
  redis:
    type: redis
`)
	worker.buildImage = func(context.Context, string, string) error {
		t.Fatal("host build should not run when job build is available")
		return nil
	}
	worker.pushImage = func(context.Context, string, string) error {
		t.Fatal("host push should not run when job build is available")
		return nil
	}

	if err := worker.Process(context.Background(), "build-1"); err != nil {
		t.Fatalf("Process() = %v, want nil", err)
	}

	got := store.builds["build-1"]
	if got.Status != StatusSucceeded {
		t.Fatalf("status = %q, want %q", got.Status, StatusSucceeded)
	}
	if got.Image != "localhost:5000/atlas/app-1:build-1" {
		t.Fatalf("image = %q, want %q", got.Image, "localhost:5000/atlas/app-1:build-1")
	}
	if deployer.buildJobOpts.BuildID != "build-1" {
		t.Fatalf("BuildID = %q, want build-1", deployer.buildJobOpts.BuildID)
	}
	if deployer.buildJobOpts.Image != "localhost:5000/atlas/app-1:build-1" {
		t.Fatalf("Image = %q, want localhost:5000/atlas/app-1:build-1", deployer.buildJobOpts.Image)
	}
	if deployer.buildJobOpts.RegistrySecretName != "registry-creds" {
		t.Fatalf("RegistrySecretName = %q, want registry-creds", deployer.buildJobOpts.RegistrySecretName)
	}
	if !deployer.buildJobOpts.InsecureRegistry {
		t.Fatal("InsecureRegistry = false, want true")
	}
	if deployer.waitBuildID != "build-1" {
		t.Fatalf("wait build id = %q, want build-1", deployer.waitBuildID)
	}
	if len(deployer.namespaces) != 1 || deployer.namespaces[0].Name != "atlas-portfolio" {
		t.Fatalf("namespaces = %+v", deployer.namespaces)
	}

	var appDeploy, redisDeploy bool
	for _, d := range deployer.deployments {
		if d.Name == "app" {
			appDeploy = true
			if d.Namespace != "atlas-portfolio" {
				t.Fatalf("app ns = %q", d.Namespace)
			}
			env := map[string]string{}
			for _, e := range d.Env {
				env[e.Name] = e.Value
			}
			if env["PORT"] != "8080" || env["REDIS_URL"] != "redis://redis:6379" {
				t.Fatalf("app env = %v", env)
			}
		}
		if d.Name == "redis" {
			redisDeploy = true
		}
	}
	if !appDeploy || !redisDeploy {
		t.Fatalf("deployments = %+v", deployer.deployments)
	}
	if len(deployer.ingresses) != 1 || deployer.ingresses[0].Host != "portfolio.edwardscott.dev" {
		t.Fatalf("ingresses = %+v", deployer.ingresses)
	}
	// Legacy teardown of portfolio in system namespace.
	foundLegacy := false
	for _, d := range deployer.deletedDeploys {
		if d == "default/portfolio" {
			foundLegacy = true
		}
	}
	if !foundLegacy {
		t.Fatalf("expected legacy delete of default/portfolio, got %v", deployer.deletedDeploys)
	}
}

func TestWorker_ProcessMissingAtlasYAML(t *testing.T) {
	now := time.Now()
	store := newFakeBuildStore(Build{
		ID: "build-1", AppID: "app-1", Status: StatusPending, CreatedAt: now, UpdatedAt: now,
	})
	store.repos = map[string]app.Repo{"app-1": {URL: "https://github.com/user/repo", Branch: "main"}}
	store.apps = map[string]app.App{"app-1": {ID: "app-1", Name: "demo"}}
	worker := NewWorker(store, WorkerConfig{Registry: "localhost:5000"}, nil, nil)
	worker.clone = func(_ context.Context, _, _, dest string) error {
		return os.MkdirAll(dest, 0o755)
	}
	worker.buildImage = func(context.Context, string, string) error { return nil }
	worker.pushImage = func(context.Context, string, string) error { return nil }

	err := worker.Process(context.Background(), "build-1")
	if err == nil {
		t.Fatal("expected error for missing atlas.yaml")
	}
	if !errors.Is(err, config.ErrMissing) && store.builds["build-1"].Status != StatusFailed {
		t.Fatalf("status = %q err = %v", store.builds["build-1"].Status, err)
	}
	if store.builds["build-1"].Status != StatusFailed {
		t.Fatalf("status = %q, want failed", store.builds["build-1"].Status)
	}
}

func TestApplyPlan_PrunesRemovedRedis(t *testing.T) {
	deployer := &fakeDeployer{
		depDeploys: []appsv1.Deployment{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "redis"},
			},
		},
		depServices: []corev1.Service{
			{ObjectMeta: metav1.ObjectMeta{Name: "redis"}},
		},
	}
	worker := NewWorker(newFakeBuildStore(), WorkerConfig{}, deployer, nil)

	p, err := plan.Build(plan.BuildOptions{
		ProjectID:   "pid",
		ProjectName: "demo",
		Image:       "img:tag",
		Config: config.Config{
			Version: 1,
			App:     config.AppConfig{Port: 8080},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := worker.ApplyPlan(context.Background(), p); err != nil {
		t.Fatalf("ApplyPlan() = %v", err)
	}

	if len(deployer.deletedDeploys) != 1 || deployer.deletedDeploys[0] != "atlas-demo/redis" {
		t.Fatalf("deletedDeploys = %v", deployer.deletedDeploys)
	}
	if len(deployer.deletedServices) != 1 || deployer.deletedServices[0] != "atlas-demo/redis" {
		t.Fatalf("deletedServices = %v", deployer.deletedServices)
	}
}

func TestWorker_ProcessNonPendingBuild(t *testing.T) {
	now := time.Now()
	store := newFakeBuildStore(Build{
		ID:        "build-1",
		AppID:     "app-1",
		Status:    StatusSucceeded,
		CreatedAt: now,
		UpdatedAt: now,
	})
	worker := NewWorker(store, WorkerConfig{}, nil, nil)

	if err := worker.Process(context.Background(), "build-1"); err != nil {
		t.Fatalf("Process() = %v, want nil", err)
	}

	got := store.builds["build-1"]
	if got.Status != StatusSucceeded {
		t.Fatalf("status = %q, want unchanged %q", got.Status, StatusSucceeded)
	}
}

func TestWorker_ProcessBuildNotFound(t *testing.T) {
	worker := NewWorker(newFakeBuildStore(), WorkerConfig{}, nil, nil)

	err := worker.Process(context.Background(), "missing")
	if err == nil {
		t.Fatal("Process() = nil, want error")
	}
}
