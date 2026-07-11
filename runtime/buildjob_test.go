package runtime

import "testing"

func TestDesiredBuildJob(t *testing.T) {
	job := desiredBuildJob(BuildJobOptions{
		BuildID:  "build-1",
		RepoURL:  "https://github.com/user/repo",
		Branch:   "main",
		Image:    "localhost:5000/atlas/app-1:build-1",
		GitImage: defaultGitImage,
	})

	if job.Name != "atlas-build-build-1" {
		t.Fatalf("Name = %q, want atlas-build-build-1", job.Name)
	}
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 0 {
		t.Fatalf("BackoffLimit = %v, want 0", job.Spec.BackoffLimit)
	}
	if len(job.Spec.Template.Spec.InitContainers) != 1 {
		t.Fatalf("init containers = %d, want 1", len(job.Spec.Template.Spec.InitContainers))
	}

	clone := job.Spec.Template.Spec.InitContainers[0]
	if clone.Image != defaultGitImage {
		t.Fatalf("clone image = %q, want %q", clone.Image, defaultGitImage)
	}
	if len(clone.Args) != 7 || clone.Args[1] != "main" || clone.Args[6] != "/workspace" {
		t.Fatalf("clone args = %v", clone.Args)
	}

	if len(job.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("containers = %d, want 1", len(job.Spec.Template.Spec.Containers))
	}
	kaniko := job.Spec.Template.Spec.Containers[0]
	if kaniko.Image != defaultKanikoImage {
		t.Fatalf("kaniko image = %q, want %q", kaniko.Image, defaultKanikoImage)
	}

	wantArgs := []string{
		"--dockerfile=/workspace/Dockerfile",
		"--context=/workspace",
		"--destination=localhost:5000/atlas/app-1:build-1",
	}
	if len(kaniko.Args) != len(wantArgs) {
		t.Fatalf("kaniko args = %v, want %v", kaniko.Args, wantArgs)
	}
	for i := range wantArgs {
		if kaniko.Args[i] != wantArgs[i] {
			t.Fatalf("kaniko args = %v, want %v", kaniko.Args, wantArgs)
		}
	}
}

func TestKanikoArgsInsecureRegistry(t *testing.T) {
	args := kanikoArgs(BuildJobOptions{
		Image:            "localhost:5000/atlas/app:build",
		InsecureRegistry: true,
	})
	if len(args) != 5 {
		t.Fatalf("args = %v, want 5 entries", args)
	}
	if args[3] != "--insecure" || args[4] != "--skip-tls-verify" {
		t.Fatalf("args = %v, want insecure flags", args)
	}
}

func TestDesiredBuildJobRegistrySecret(t *testing.T) {
	job := desiredBuildJob(BuildJobOptions{
		BuildID:            "build-1",
		RepoURL:            "https://github.com/user/repo",
		Branch:             "main",
		Image:              "localhost:5000/atlas/app:build",
		RegistrySecretName: "registry-creds",
	})

	volumes := job.Spec.Template.Spec.Volumes
	if len(volumes) != 2 {
		t.Fatalf("volumes = %d, want 2", len(volumes))
	}
	if volumes[1].Secret == nil || volumes[1].Secret.SecretName != "registry-creds" {
		t.Fatalf("registry secret volume = %+v", volumes[1])
	}

	kaniko := job.Spec.Template.Spec.Containers[0]
	if len(kaniko.VolumeMounts) != 2 {
		t.Fatalf("kaniko volume mounts = %d, want 2", len(kaniko.VolumeMounts))
	}
	if kaniko.VolumeMounts[1].MountPath != "/kaniko/.docker" {
		t.Fatalf("docker config mount = %q, want /kaniko/.docker", kaniko.VolumeMounts[1].MountPath)
	}
}

func TestBuildJobName(t *testing.T) {
	if got := buildJobName("abc-123"); got != "atlas-build-abc-123" {
		t.Fatalf("buildJobName() = %q, want atlas-build-abc-123", got)
	}
}

func TestValidateBuildJobOptions(t *testing.T) {
	err := validateBuildJobOptions(BuildJobOptions{})
	if err == nil {
		t.Fatal("validateBuildJobOptions() = nil, want error")
	}

	err = validateBuildJobOptions(BuildJobOptions{
		BuildID: "build-1",
		RepoURL: "https://github.com/user/repo",
		Branch:  "main",
		Image:   "localhost:5000/atlas/app:build",
	})
	if err != nil {
		t.Fatalf("validateBuildJobOptions() = %v, want nil", err)
	}
}
