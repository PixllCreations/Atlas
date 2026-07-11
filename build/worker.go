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
	EnsureIngress(ctx context.Context, opts runtime.IngressOptions) error
}

// WorkerConfig holds runtime settings for the build worker.
type WorkerConfig struct {
	Registry      string
	Namespace     string
	IngressDomain string
	IngressClass  string
}

// Worker executes builds and updates their lifecycle status.
type Worker struct {
	cfg        WorkerConfig
	store      Store
	deployer   Deployer
	clone      func(ctx context.Context, url, branch, dest string) error
	buildImage func(ctx context.Context, contextDir, imageTag string) error
	pushImage  func(ctx context.Context, registry, imageTag string) error
}

func NewWorker(store Store, cfg WorkerConfig, deployer Deployer) *Worker {
	return NewWorkerWithHooks(store, cfg, deployer, CloneRepo, BuildImage, PushImage)
}

func NewWorkerWithClone(store Store, cfg WorkerConfig, clone func(ctx context.Context, url, branch, dest string) error) *Worker {
	return NewWorkerWithHooks(store, cfg, nil, clone, BuildImage, PushImage)
}

func NewWorkerWithHooks(
	store Store,
	cfg WorkerConfig,
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
		cfg:        cfg,
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

	if w.cfg.Registry != "" {
		if err := w.pushImage(ctx, w.cfg.Registry, tag); err != nil {
			return err
		}

		if w.deployer != nil {
			a, err := w.store.GetApp(ctx, b.AppID)
			if err != nil {
				return fmt.Errorf("get app: %w", err)
			}

			if err := w.deployer.EnsureDeployment(ctx, runtime.DeployOptions{
				Namespace: w.cfg.Namespace,
				Name:      a.Name,
				Image:     RemoteImageTag(w.cfg.Registry, tag),
			}); err != nil {
				return fmt.Errorf("deploy: %w", err)
			}

			if err := w.deployer.EnsureService(ctx, runtime.ServiceOptions{
				Namespace: w.cfg.Namespace,
				Name:      a.Name,
			}); err != nil {
				return fmt.Errorf("service: %w", err)
			}

			if w.cfg.IngressDomain != "" {
				if err := w.deployer.EnsureIngress(ctx, runtime.IngressOptions{
					Namespace:        w.cfg.Namespace,
					Name:             a.Name,
					Host:             ingressHost(a.Name, w.cfg.IngressDomain),
					IngressClassName: w.cfg.IngressClass,
				}); err != nil {
					return fmt.Errorf("ingress: %w", err)
				}
			}
		}
	}

	return nil
}

func imageTag(b Build) string {
	return fmt.Sprintf("atlas/%s:%s", b.AppID, b.ID)
}

func ingressHost(appName, domain string) string {
	return appName + "." + domain
}
