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
	UpdateBuildImage(ctx context.Context, id string, image string) (Build, error)
	GetRepo(ctx context.Context, appID string) (app.Repo, error)
	GetApp(ctx context.Context, id string) (app.App, error)
}

// GitHubCloner resolves authenticated clone URLs for GitHub App installations.
type GitHubCloner interface {
	CloneURL(ctx context.Context, installationID int64, fullName string) (string, error)
}

// Deployer applies builds and deploys to the runtime cluster.
type Deployer interface {
	EnsureBuildJob(ctx context.Context, opts runtime.BuildJobOptions) error
	WaitForBuildJob(ctx context.Context, namespace, buildID string) error
	EnsureDeployment(ctx context.Context, opts runtime.DeployOptions) error
	EnsureService(ctx context.Context, opts runtime.ServiceOptions) error
	EnsureIngress(ctx context.Context, opts runtime.IngressOptions) error
	DeleteDeployment(ctx context.Context, namespace, name string) error
	DeleteService(ctx context.Context, namespace, name string) error
	DeleteIngress(ctx context.Context, namespace, name string) error
}

// WorkerConfig holds runtime settings for the build worker.
type WorkerConfig struct {
	Registry           string
	Namespace          string
	IngressDomain      string
	IngressClass       string
	IngressTLSSecret   string
	RegistrySecretName string
	InsecureRegistry   bool
}

// Worker executes builds and updates their lifecycle status.
type Worker struct {
	cfg        WorkerConfig
	store      Store
	deployer   Deployer
	github     GitHubCloner
	clone      func(ctx context.Context, url, branch, dest string) error
	buildImage func(ctx context.Context, contextDir, imageTag string) error
	pushImage  func(ctx context.Context, registry, imageTag string) error
}

func NewWorker(store Store, cfg WorkerConfig, deployer Deployer, gh GitHubCloner) *Worker {
	return NewWorkerWithHooks(store, cfg, deployer, gh, CloneRepo, BuildImage, PushImage)
}

func NewWorkerWithClone(store Store, cfg WorkerConfig, clone func(ctx context.Context, url, branch, dest string) error) *Worker {
	return NewWorkerWithHooks(store, cfg, nil, nil, clone, BuildImage, PushImage)
}

func NewWorkerWithHooks(
	store Store,
	cfg WorkerConfig,
	deployer Deployer,
	gh GitHubCloner,
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
		github:     gh,
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

	cloneURL, err := w.cloneURL(ctx, repo)
	if err != nil {
		return err
	}

	tag := imageTag(b)
	var remote string

	if w.useJobBuild() {
		remote = RemoteImageTag(w.cfg.Registry, tag)
		if err := w.runJobBuild(ctx, b, repo, cloneURL, remote); err != nil {
			return err
		}
	} else {
		remote, err = w.runHostBuild(ctx, b, repo, cloneURL, tag)
		if err != nil {
			return err
		}
	}

	if remote == "" {
		return nil
	}

	if _, err := w.store.UpdateBuildImage(ctx, b.ID, remote); err != nil {
		return fmt.Errorf("save image: %w", err)
	}

	return w.deployApp(ctx, b, remote)
}

func (w *Worker) cloneURL(ctx context.Context, repo app.Repo) (string, error) {
	if repo.InstallationID != 0 && repo.GitHubFullName != "" && w.github != nil {
		url, err := w.github.CloneURL(ctx, repo.InstallationID, repo.GitHubFullName)
		if err != nil {
			return "", fmt.Errorf("github clone url: %w", err)
		}
		return url, nil
	}
	return repo.URL, nil
}

func (w *Worker) useJobBuild() bool {
	return w.deployer != nil && w.cfg.Registry != ""
}

func (w *Worker) runHostBuild(ctx context.Context, b Build, repo app.Repo, cloneURL, tag string) (string, error) {
	dir, err := os.MkdirTemp("", "atlas-build-*")
	if err != nil {
		return "", fmt.Errorf("create workdir: %w", err)
	}
	defer os.RemoveAll(dir)

	src := filepath.Join(dir, "src")
	if err := w.clone(ctx, cloneURL, repo.Branch, src); err != nil {
		return "", err
	}

	if err := w.buildImage(ctx, src, tag); err != nil {
		return "", err
	}

	if w.cfg.Registry == "" {
		return "", nil
	}

	if err := w.pushImage(ctx, w.cfg.Registry, tag); err != nil {
		return "", err
	}

	return RemoteImageTag(w.cfg.Registry, tag), nil
}

func (w *Worker) runJobBuild(ctx context.Context, b Build, repo app.Repo, cloneURL, remote string) error {
	if err := w.deployer.EnsureBuildJob(ctx, runtime.BuildJobOptions{
		Namespace:          w.cfg.Namespace,
		BuildID:            b.ID,
		RepoURL:            cloneURL,
		Branch:             repo.Branch,
		Image:              remote,
		RegistrySecretName: w.cfg.RegistrySecretName,
		InsecureRegistry:   w.cfg.InsecureRegistry,
	}); err != nil {
		return fmt.Errorf("create build job: %w", err)
	}

	if err := w.deployer.WaitForBuildJob(ctx, w.cfg.Namespace, b.ID); err != nil {
		return fmt.Errorf("build job: %w", err)
	}
	return nil
}

func (w *Worker) deployApp(ctx context.Context, b Build, remote string) error {
	if w.deployer == nil {
		return nil
	}

	a, err := w.store.GetApp(ctx, b.AppID)
	if err != nil {
		return fmt.Errorf("get app: %w", err)
	}

	containerPort := int32(a.Port)
	if containerPort <= 0 {
		containerPort = int32(app.DefaultPort)
	}

	if err := w.deployer.EnsureDeployment(ctx, runtime.DeployOptions{
		Namespace: w.cfg.Namespace,
		Name:      a.Name,
		Image:     remote,
		Port:      containerPort,
	}); err != nil {
		return fmt.Errorf("deploy: %w", err)
	}

	if err := w.deployer.EnsureService(ctx, runtime.ServiceOptions{
		Namespace:     w.cfg.Namespace,
		Name:          a.Name,
		Port:          80,
		ContainerPort: containerPort,
	}); err != nil {
		return fmt.Errorf("service: %w", err)
	}

	if w.cfg.IngressDomain != "" {
		if err := w.deployer.EnsureIngress(ctx, runtime.IngressOptions{
			Namespace:        w.cfg.Namespace,
			Name:             a.Name,
			Host:             ingressHost(a.Name, w.cfg.IngressDomain),
			Port:             80,
			IngressClassName: w.cfg.IngressClass,
			TLSSecretName:    w.cfg.IngressTLSSecret,
		}); err != nil {
			return fmt.Errorf("ingress: %w", err)
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
