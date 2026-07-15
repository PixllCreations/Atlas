package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultKanikoImage = "gcr.io/kaniko-project/executor:v1.23.2"
	defaultGitImage    = "alpine/git:latest"
	buildWorkspaceVol  = "workspace"
	buildDockerCfgVol  = "docker-config"
)

// BuildJobOptions configures a Kubernetes Job that clones source and builds with Kaniko.
type BuildJobOptions struct {
	Namespace          string
	BuildID            string
	RepoURL            string
	Branch             string
	Image              string
	KanikoImage        string
	GitImage           string
	RegistrySecretName string
	InsecureRegistry   bool
}

// EnsureBuildJob creates a build Job for the given build ID.
func (c *Client) EnsureBuildJob(ctx context.Context, opts BuildJobOptions) error {
	if err := validateBuildJobOptions(opts); err != nil {
		return err
	}
	if opts.Namespace == "" {
		opts.Namespace = "default"
	}

	job := desiredBuildJob(opts)
	jobs := c.clientset.BatchV1().Jobs(opts.Namespace)

	_, err := jobs.Create(ctx, job, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create build job: %w", err)
	}
	return nil
}

// WaitForBuildJob blocks until the build Job succeeds or fails.
// onLogs, when non-nil, is called periodically with the full current Job log snapshot.
func (c *Client) WaitForBuildJob(ctx context.Context, namespace, buildID string, onLogs func(string)) error {
	if buildID == "" {
		return fmt.Errorf("build job: build id is required")
	}
	if namespace == "" {
		namespace = "default"
	}

	name := buildJobName(buildID)
	jobs := c.clientset.BatchV1().Jobs(namespace)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	emitLogs := func() {
		if onLogs == nil {
			return
		}
		logs, err := c.TailBuildJobLogs(ctx, namespace, buildID)
		if err != nil || logs == "" {
			return
		}
		onLogs(logs)
	}

	for {
		job, err := jobs.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("get build job: %w", err)
		}

		emitLogs()

		if job.Status.Succeeded > 0 {
			emitLogs()
			return nil
		}
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == corev1.ConditionTrue {
				emitLogs()
				return nil
			}
			if cond.Type == batchv1.JobFailed && cond.Status == corev1.ConditionTrue {
				emitLogs()
				msg := strings.TrimSpace(cond.Message)
				if msg == "" {
					msg = "build job failed"
				}
				return fmt.Errorf("%s", msg)
			}
		}
		if job.Status.Failed > 0 {
			emitLogs()
			return fmt.Errorf("build job failed")
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func validateBuildJobOptions(opts BuildJobOptions) error {
	if opts.BuildID == "" {
		return fmt.Errorf("build job: build id is required")
	}
	if strings.TrimSpace(opts.RepoURL) == "" {
		return fmt.Errorf("build job: repo url is required")
	}
	if strings.TrimSpace(opts.Branch) == "" {
		return fmt.Errorf("build job: branch is required")
	}
	if strings.TrimSpace(opts.Image) == "" {
		return fmt.Errorf("build job: image is required")
	}
	return nil
}

func desiredBuildJob(opts BuildJobOptions) *batchv1.Job {
	name := buildJobName(opts.BuildID)
	labels := map[string]string{
		"app.kubernetes.io/managed-by": "atlas",
		"atlas.build/id":               opts.BuildID,
	}

	gitImage := opts.GitImage
	if gitImage == "" {
		gitImage = defaultGitImage
	}
	kanikoImage := opts.KanikoImage
	if kanikoImage == "" {
		kanikoImage = defaultKanikoImage
	}

	volumes := []corev1.Volume{
		{
			Name: buildWorkspaceVol,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		},
	}
	volumeMounts := []corev1.VolumeMount{
		{
			Name:      buildWorkspaceVol,
			MountPath: "/workspace",
		},
	}

	if opts.RegistrySecretName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: buildDockerCfgVol,
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: opts.RegistrySecretName,
					Items: []corev1.KeyToPath{
						{Key: ".dockerconfigjson", Path: "config.json"},
					},
				},
			},
		})
		volumeMounts = append(volumeMounts, corev1.VolumeMount{
			Name:      buildDockerCfgVol,
			MountPath: "/kaniko/.docker",
			ReadOnly:  true,
		})
	}

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: int32Ptr(0),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					InitContainers: []corev1.Container{
						{
							Name:    "clone",
							Image:   gitImage,
							Command: []string{"git", "clone"},
							Args: []string{
								"--branch", opts.Branch,
								"--depth", "1",
								"--single-branch",
								opts.RepoURL,
								"/workspace",
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      buildWorkspaceVol,
									MountPath: "/workspace",
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:         "kaniko",
							Image:        kanikoImage,
							Args:         kanikoArgs(opts),
							VolumeMounts: volumeMounts,
						},
					},
					Volumes: volumes,
				},
			},
		},
	}
}

func kanikoArgs(opts BuildJobOptions) []string {
	args := []string{
		"--dockerfile=/workspace/Dockerfile",
		"--context=/workspace",
		"--destination=" + opts.Image,
	}
	if opts.InsecureRegistry {
		args = append(args, "--insecure", "--skip-tls-verify")
	}
	return args
}

func buildJobName(buildID string) string {
	return "atlas-build-" + buildID
}
