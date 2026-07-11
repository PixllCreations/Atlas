package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pixll/atlas/app"
)

type Store interface {
	GetBuild(ctx context.Context, id string) (Build, error)
	UpdateBuildStatus(ctx context.Context, id string, status Status) (Build, error)
	GetRepo(ctx context.Context, appID string) (app.Repo, error)
}

// Worker executes builds and updates their lifecycle status.
type Worker struct {
	store      Store
	clone      func(ctx context.Context, url, branch, dest string) error
	buildImage func(ctx context.Context, contextDir, imageTag string) error
}

func NewWorker(store Store) *Worker {
	return NewWorkerWithHooks(store, CloneRepo, BuildImage)
}

func NewWorkerWithClone(store Store, clone func(ctx context.Context, url, branch, dest string) error) *Worker {
	return NewWorkerWithHooks(store, clone, BuildImage)
}

func NewWorkerWithHooks(
	store Store,
	clone func(ctx context.Context, url, branch, dest string) error,
	buildImage func(ctx context.Context, contextDir, imageTag string) error,
) *Worker {
	if clone == nil {
		clone = CloneRepo
	}
	if buildImage == nil {
		buildImage = BuildImage
	}
	return &Worker{store: store, clone: clone, buildImage: buildImage}
}

// Process runs a single build by ID.
func (w *Worker) Process(ctx context.Context, buildID string) error {
	b, err := w.store.GetBuild(ctx, buildID)
	if err != nil {
		return fmt.Errorf("get build: %w", err)
	}
	if b.Status != StatusPending {
		return nil
	}

	if _, err := w.store.UpdateBuildStatus(ctx, buildID, StatusRunning); err != nil {
		return fmt.Errorf("mark running: %w", err)
	}

	if err := w.execute(ctx, b); err != nil {
		if _, updateErr := w.store.UpdateBuildStatus(ctx, buildID, StatusFailed); updateErr != nil {
			return fmt.Errorf("run build: %w (mark failed: %v)", err, updateErr)
		}
		return fmt.Errorf("run build: %w", err)
	}

	if _, err := w.store.UpdateBuildStatus(ctx, buildID, StatusSucceeded); err != nil {
		return fmt.Errorf("mark succeeded: %w", err)
	}
	return nil
}

func (w *Worker) execute(ctx context.Context, b Build) error {
	repo, err := w.store.GetRepo(ctx, b.AppID)
	if err != nil {
		return fmt.Errorf("get repo: %w", err)
	}

	dir, err := os.MkdirTemp("", "atlas-build-*")
	if err != nil {
		return fmt.Errorf("create workdir: %w", err)
	}
	defer os.RemoveAll(dir)

	src := filepath.Join(dir, "src")
	if err := w.clone(ctx, repo.URL, repo.Branch, src); err != nil {
		return err
	}

	if err := w.buildImage(ctx, src, imageTag(b)); err != nil {
		return err
	}

	// Registry push comes next.
	return nil
}

func imageTag(b Build) string {
	return fmt.Sprintf("atlas/%s:%s", b.AppID, b.ID)
}
