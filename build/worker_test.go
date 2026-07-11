package build

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/pixll/atlas/app"
)

type fakeBuildStore struct {
	builds map[string]Build
	repos  map[string]app.Repo
}

func newFakeBuildStore(builds ...Build) *fakeBuildStore {
	m := make(map[string]Build, len(builds))
	for _, b := range builds {
		m[b.ID] = b
	}
	return &fakeBuildStore{builds: m}
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
	worker := NewWorker(store, "localhost:5000")
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
	worker := NewWorker(store, "")

	if err := worker.Process(context.Background(), "build-1"); err != nil {
		t.Fatalf("Process() = %v, want nil", err)
	}

	got := store.builds["build-1"]
	if got.Status != StatusSucceeded {
		t.Fatalf("status = %q, want unchanged %q", got.Status, StatusSucceeded)
	}
}

func TestWorker_ProcessBuildNotFound(t *testing.T) {
	worker := NewWorker(newFakeBuildStore(), "")

	err := worker.Process(context.Background(), "missing")
	if err == nil {
		t.Fatal("Process() = nil, want error")
	}
}
