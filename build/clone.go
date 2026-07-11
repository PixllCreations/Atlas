package build

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// CloneRepo clones url at branch into dest using a shallow single-branch git clone.
func CloneRepo(ctx context.Context, url, branch, dest string) error {
	url = strings.TrimSpace(url)
	branch = strings.TrimSpace(branch)
	if url == "" {
		return fmt.Errorf("clone repo: url is required")
	}
	if branch == "" {
		return fmt.Errorf("clone repo: branch is required")
	}
	if dest == "" {
		return fmt.Errorf("clone repo: dest is required")
	}

	cmd := exec.CommandContext(ctx, "git", "clone",
		"--branch", branch,
		"--depth", "1",
		"--single-branch",
		url, dest,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
