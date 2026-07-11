package build

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// PushImage retags imageTag for registry and pushes it.
// Example: PushImage(ctx, "localhost:5000", "atlas/app:build") pushes localhost:5000/atlas/app:build.
func PushImage(ctx context.Context, registry, imageTag string) error {
	registry = strings.TrimSpace(registry)
	registry = strings.TrimRight(registry, "/")
	imageTag = strings.TrimSpace(imageTag)
	if registry == "" {
		return fmt.Errorf("push image: registry is required")
	}
	if imageTag == "" {
		return fmt.Errorf("push image: image tag is required")
	}

	remoteTag := registry + "/" + imageTag

	tagCmd := exec.CommandContext(ctx, "docker", "tag", imageTag, remoteTag)
	if out, err := tagCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker tag: %w: %s", err, strings.TrimSpace(string(out)))
	}

	pushCmd := exec.CommandContext(ctx, "docker", "push", remoteTag)
	if out, err := pushCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("docker push: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemoteImageTag returns the registry-qualified tag for imageTag.
func RemoteImageTag(registry, imageTag string) string {
	registry = strings.TrimSpace(registry)
	registry = strings.TrimRight(registry, "/")
	imageTag = strings.TrimSpace(imageTag)
	return registry + "/" + imageTag
}
