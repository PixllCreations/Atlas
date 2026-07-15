package config

import (
	"fmt"
	"regexp"
	"strings"
)

// Kubernetes DNS subdomain label: lowercase alphanumeric, hyphens, max 63 chars.
var dependencyNamePattern = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// Validate checks that cfg is a supported Atlas repository configuration.
func Validate(cfg Config) error {
	if cfg.Version != SupportedVersion {
		return fmt.Errorf("unsupported atlas.yaml version %d; supported version is %d", cfg.Version, SupportedVersion)
	}
	if cfg.App.Port < 1 || cfg.App.Port > 65535 {
		return fmt.Errorf("app.port must be between 1 and 65535, got %d", cfg.App.Port)
	}
	if cfg.Dependencies == nil {
		return nil
	}

	var redisCount int
	for name, dep := range cfg.Dependencies {
		if err := validateDependencyName(name); err != nil {
			return err
		}
		switch dep.Type {
		case DependencyRedis:
			redisCount++
			if redisCount > 1 {
				return fmt.Errorf("only one redis dependency is allowed per project; found multiple")
			}
		case DependencyPostgres:
			return fmt.Errorf("dependency %q: type %q is not yet supported", name, dep.Type)
		case DependencyNATS:
			return fmt.Errorf("dependency %q: type %q is not yet supported", name, dep.Type)
		case "":
			return fmt.Errorf("dependency %q: type is required", name)
		default:
			return fmt.Errorf("dependency %q: unsupported type %q", name, dep.Type)
		}
	}
	return nil
}

func validateDependencyName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("dependency name is required")
	}
	if name == "app" {
		return fmt.Errorf("dependency name %q is reserved for the primary application", name)
	}
	if len(name) > 63 {
		return fmt.Errorf("dependency name %q exceeds 63 characters", name)
	}
	if !dependencyNamePattern.MatchString(name) {
		return fmt.Errorf("dependency name %q is not a valid Kubernetes resource name (use lowercase alphanumeric and hyphens)", name)
	}
	return nil
}
