package build

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	"github.com/pixll/atlas/app"
	"github.com/pixll/atlas/config"
	"github.com/pixll/atlas/dependency"
	depcredis "github.com/pixll/atlas/dependency/redis"
	"github.com/pixll/atlas/plan"
	"github.com/pixll/atlas/runtime"
)

type Store interface {
	GetBuild(ctx context.Context, id string) (Build, error)
	UpdateBuildStatus(ctx context.Context, id string, status Status) (Build, error)
	UpdateBuildPhase(ctx context.Context, id string, phase Phase) (Build, error)
	UpdateBuildImage(ctx context.Context, id string, image string) (Build, error)
	AppendBuildLog(ctx context.Context, id string, chunk string) error
	GetRepo(ctx context.Context, appID string) (app.Repo, error)
	GetApp(ctx context.Context, id string) (app.App, error)
	UpdateAppDeploymentSnapshot(ctx context.Context, id string, snapshot []byte) error
}

// GitHubCloner resolves authenticated clone URLs for GitHub App installations.
type GitHubCloner interface {
	CloneURL(ctx context.Context, installationID int64, fullName string) (string, error)
}

// Deployer applies builds and deploys to the runtime cluster.
type Deployer interface {
	EnsureBuildJob(ctx context.Context, opts runtime.BuildJobOptions) error
	WaitForBuildJob(ctx context.Context, namespace, buildID string, onLogs func(string)) error
	TailBuildJobLogs(ctx context.Context, namespace, buildID string) (string, error)
	EnsureNamespace(ctx context.Context, opts runtime.NamespaceOptions) error
	DeleteNamespace(ctx context.Context, name string) error
	EnsureDeployment(ctx context.Context, opts runtime.DeployOptions) error
	EnsureService(ctx context.Context, opts runtime.ServiceOptions) error
	EnsureIngress(ctx context.Context, opts runtime.IngressOptions) error
	DeleteDeployment(ctx context.Context, namespace, name string) error
	DeleteService(ctx context.Context, namespace, name string) error
	DeleteIngress(ctx context.Context, namespace, name string) error
	ListManagedDependencyDeployments(ctx context.Context, namespace, projectID string) ([]appsv1.Deployment, error)
	ListManagedDependencyServices(ctx context.Context, namespace, projectID string) ([]corev1.Service, error)
}

// WorkerConfig holds runtime settings for the build worker.
type WorkerConfig struct {
	Registry           string
	Namespace          string // system/build namespace (ATLAS_K8S_NAMESPACE)
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
	deps       *dependency.Registry
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
	reg := dependency.NewRegistry()
	if deployer != nil {
		reg.Register(config.DependencyRedis, depcredis.NewProvisioner(deployer))
	}
	return &Worker{
		cfg:        cfg,
		store:      store,
		deployer:   deployer,
		github:     gh,
		deps:       reg,
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
		_ = w.appendLog(ctx, buildID, "error: "+err.Error()+"\n")
		if _, updateErr := w.store.UpdateBuildStatus(ctx, buildID, StatusFailed); updateErr != nil {
			return fmt.Errorf("run build: %w (mark failed: %v)", err, updateErr)
		}
		return fmt.Errorf("run build: %w", err)
	}

	if _, err := w.store.UpdateBuildStatus(ctx, buildID, StatusSucceeded); err != nil {
		return fmt.Errorf("mark succeeded: %w", err)
	}
	_ = w.appendLog(ctx, buildID, "build succeeded\n")
	return nil
}

func (w *Worker) execute(ctx context.Context, b Build) error {
	if err := w.setPhase(ctx, b.ID, PhaseCloning); err != nil {
		return err
	}
	_ = w.appendLog(ctx, b.ID, "cloning repository\n")

	repo, err := w.store.GetRepo(ctx, b.AppID)
	if err != nil {
		return fmt.Errorf("get repo: %w", err)
	}

	a, err := w.store.GetApp(ctx, b.AppID)
	if err != nil {
		return fmt.Errorf("get app: %w", err)
	}

	cloneURL, err := w.cloneURL(ctx, repo)
	if err != nil {
		return err
	}

	dir, err := os.MkdirTemp("", "atlas-build-*")
	if err != nil {
		return fmt.Errorf("create workdir: %w", err)
	}
	defer os.RemoveAll(dir)

	src := filepath.Join(dir, "src")
	if err := w.clone(ctx, cloneURL, repo.Branch, src); err != nil {
		return err
	}
	_ = w.appendLog(ctx, b.ID, "cloned; reading atlas.yaml\n")

	cfg, err := config.Load(filepath.Join(src, "atlas.yaml"))
	if err != nil {
		return err
	}
	_ = w.appendLog(ctx, b.ID, "atlas.yaml ok\n")

	tag := imageTag(b)
	var remote string

	if err := w.setPhase(ctx, b.ID, PhaseBuilding); err != nil {
		return err
	}

	if w.useJobBuild() {
		remote = RemoteImageTag(w.cfg.Registry, tag)
		_ = w.appendLog(ctx, b.ID, "starting Kaniko build job\n")
		if err := w.runJobBuild(ctx, b, repo, cloneURL, remote); err != nil {
			return err
		}
		_ = w.appendLog(ctx, b.ID, "image build complete: "+remote+"\n")
	} else {
		_ = w.appendLog(ctx, b.ID, "building image on host\n")
		if err := w.buildImage(ctx, src, tag); err != nil {
			return err
		}
		if w.cfg.Registry == "" {
			_ = w.appendLog(ctx, b.ID, "no registry configured; skipping push/deploy\n")
			return nil
		}
		if err := w.setPhase(ctx, b.ID, PhasePushing); err != nil {
			return err
		}
		_ = w.appendLog(ctx, b.ID, "pushing image\n")
		if err := w.pushImage(ctx, w.cfg.Registry, tag); err != nil {
			return err
		}
		remote = RemoteImageTag(w.cfg.Registry, tag)
		_ = w.appendLog(ctx, b.ID, "pushed "+remote+"\n")
	}

	if _, err := w.store.UpdateBuildImage(ctx, b.ID, remote); err != nil {
		return fmt.Errorf("save image: %w", err)
	}

	if err := w.setPhase(ctx, b.ID, PhaseDeploying); err != nil {
		return err
	}
	_ = w.appendLog(ctx, b.ID, "deploying to "+plan.NamespaceName(a.Name)+"\n")

	p, err := plan.Build(plan.BuildOptions{
		ProjectID:     a.ID,
		ProjectName:   a.Name,
		Image:         remote,
		IngressDomain: w.cfg.IngressDomain,
		Config:        cfg,
	})
	if err != nil {
		return fmt.Errorf("plan: %w", err)
	}

	if err := w.ApplyPlan(ctx, p); err != nil {
		return err
	}

	w.cleanupLegacyResources(ctx, a.Name)

	if snap, err := encodeSnapshot(SnapshotFromPlan(p)); err == nil {
		if err := w.store.UpdateAppDeploymentSnapshot(ctx, a.ID, snap); err != nil {
			return fmt.Errorf("save deployment snapshot: %w", err)
		}
	} else {
		return fmt.Errorf("encode deployment snapshot: %w", err)
	}

	_ = w.appendLog(ctx, b.ID, "deploy complete\n")
	return nil
}

func (w *Worker) setPhase(ctx context.Context, buildID string, phase Phase) error {
	if _, err := w.store.UpdateBuildPhase(ctx, buildID, phase); err != nil {
		return fmt.Errorf("set phase %s: %w", phase, err)
	}
	return nil
}

func (w *Worker) appendLog(ctx context.Context, buildID, chunk string) error {
	return w.store.AppendBuildLog(ctx, buildID, chunk)
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

	var last string
	if err := w.deployer.WaitForBuildJob(ctx, w.cfg.Namespace, b.ID, func(full string) {
		if full == "" || full == last {
			return
		}
		chunk := full
		if strings.HasPrefix(full, last) {
			chunk = full[len(last):]
		}
		last = full
		_ = w.appendLog(ctx, b.ID, chunk)
	}); err != nil {
		return fmt.Errorf("build job: %w", err)
	}
	return nil
}

func imageTag(b Build) string {
	return fmt.Sprintf("atlas/%s:%s", b.AppID, b.ID)
}
