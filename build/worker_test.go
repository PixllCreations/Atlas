package build

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pixll/atlas/app"
	"github.com/pixll/atlas/runtime"
)

type fakeBuildStore struct {
	builds map[string]Build
	repos  map[string]app.Repo
	apps   map[string]app.App
}

func newFakeBuildStore(builds ...Build) *fakeBuildStore {
	m := make(map[string]Build, len(builds))
	for _, b := range builds {
		m[b.ID] = b
	}
	return &fakeBuildStore{builds: m}
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
	worker := NewWorker(store, WorkerConfig{Registry: "localhost:5000"}, nil)
	worker.clone = func(context.Context, string, string, string) error { return nil }
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
}

type fakeDeployer struct {
	buildJobOpts runtime.BuildJobOptions
	waitNS       string
	waitBuildID  string
}

func (f *fakeDeployer) EnsureBuildJob(_ context.Context, opts runtime.BuildJobOptions) error {
	f.buildJobOpts = opts
	return nil
}

func (f *fakeDeployer) WaitForBuildJob(_ context.Context, namespace, buildID string) error {
	f.waitNS = namespace
	f.waitBuildID = buildID
	return nil
}

func (f *fakeDeployer) EnsureDeployment(context.Context, runtime.DeployOptions) error { return nil }
func (f *fakeDeployer) EnsureService(context.Context, runtime.ServiceOptions) error    { return nil }
func (f *fakeDeployer) EnsureIngress(context.Context, runtime.IngressOptions) error   { return nil }

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
		RegistrySecretName: "registry-creds",
		InsecureRegistry:   true,
	}, deployer)
	worker.clone = func(context.Context, string, string, string) error {
		t.Fatal("host clone should not run when job build is available")
		return nil
	}
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
	worker := NewWorker(store, WorkerConfig{}, nil)

	if err := worker.Process(context.Background(), "build-1"); err != nil {
		t.Fatalf("Process() = %v, want nil", err)
	}

	got := store.builds["build-1"]
	if got.Status != StatusSucceeded {
		t.Fatalf("status = %q, want unchanged %q", got.Status, StatusSucceeded)
	}
}

func TestWorker_ProcessBuildNotFound(t *testing.T) {
	worker := NewWorker(newFakeBuildStore(), WorkerConfig{}, nil)

	err := worker.Process(context.Background(), "missing")
	if err == nil {
		t.Fatal("Process() = nil, want error")
	}
}
