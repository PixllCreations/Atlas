package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pixll/atlas/app"
	"github.com/pixll/atlas/runtime"
)

type Store interface {
	GetBuild(ctx context.Context, id string) (Build, error)
	UpdateBuildStatus(ctx context.Context, id string, status Status) (Build, error)
	GetRepo(ctx context.Context, appID string) (app.Repo, error)
	GetApp(ctx context.Context, id string) (app.App, error)
}

// Deployer applies a built image to the runtime cluster.
type Deployer interface {
	EnsureDeployment(ctx context.Context, opts runtime.DeployOptions) error
	EnsureService(ctx context.Context, opts runtime.ServiceOptions) error
}

// Worker executes builds and updates their lifecycle status.
type Worker struct {
	registry   string
	namespace  string
	store      Store
	deployer   Deployer
	clone      func(ctx context.Context, url, branch, dest string) error
	buildImage func(ctx context.Context, contextDir, imageTag string) error
	pushImage  func(ctx context.Context, registry, imageTag string) error
}

func NewWorker(store Store, registry string, deployer Deployer, namespace string) *Worker {
	return NewWorkerWithHooks(store, registry, namespace, deployer, CloneRepo, BuildImage, PushImage)
}

func NewWorkerWithClone(store Store, registry, namespace string, clone func(ctx context.Context, url, branch, dest string) error) *Worker {
	return NewWorkerWithHooks(store, registry, namespace, nil, clone, BuildImage, PushImage)
}

func NewWorkerWithHooks(
	store Store,
	registry string,
	namespace string,
	deployer Deployer,
	clone func(ctx context.Context, url, branch, dest string) error,
	buildImage func(ctx context.Context, contextDir, imageTag string) error,
	pushImage func(ctx context.Context, registry, imageTag string) error,
) *Worker {
	if clone == nil {
		clone = CloneRepo
	}
	if buildImage == nil {
		buildImage = BuildImage
	}
	if pushImage == nil {
		pushImage = PushImage
	}
	return &Worker{
		registry:   registry,
		namespace:  namespace,
		store:      store,
		deployer:   deployer,
		clone:      clone,
		buildImage: buildImage,
		pushImage:  pushImage,
	}
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

	tag := imageTag(b)
	if err := w.buildImage(ctx, src, tag); err != nil {
		return err
	}

	if w.registry != "" {
		if err := w.pushImage(ctx, w.registry, tag); err != nil {
			return err
		}

		if w.deployer != nil {
			a, err := w.store.GetApp(ctx, b.AppID)
			if err != nil {
				return fmt.Errorf("get app: %w", err)
			}

			if err := w.deployer.EnsureDeployment(ctx, runtime.DeployOptions{
				Namespace: w.namespace,
				Name:      a.Name,
				Image:     RemoteImageTag(w.registry, tag),
			}); err != nil {
				return fmt.Errorf("deploy: %w", err)
			}

			if err := w.deployer.EnsureService(ctx, runtime.ServiceOptions{
				Namespace: w.namespace,
				Name:      a.Name,
			}); err != nil {
				return fmt.Errorf("service: %w", err)
			}
		}
	}

	return nil
}

func imageTag(b Build) string {
	return fmt.Sprintf("atlas/%s:%s", b.AppID, b.ID)
}
