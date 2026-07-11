package build

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// BuildImage builds a Docker image from contextDir and tags it as imageTag.
// Expects a Dockerfile at the root of contextDir.
func BuildImage(ctx context.Context, contextDir, imageTag string) error {
	contextDir = strings.TrimSpace(contextDir)
	imageTag = strings.TrimSpace(imageTag)
	if contextDir == "" {
		return fmt.Errorf("build image: context dir is required")
	}
	if imageTag == "" {
		return fmt.Errorf("build image: image tag is required")
	}

	cmd := exec.CommandContext(ctx, "docker", "build",
		"-t", imageTag,
		contextDir,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker build: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
