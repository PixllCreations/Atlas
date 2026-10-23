package plan

import (
	"fmt"
	"regexp"
	"strings"
)

var environmentName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// ValidateEnvironment validates portable variable names and runtime value limits.
func ValidateEnvironment(env map[string]string) error {
	if len(env) > 256 {
		return fmt.Errorf("at most 256 environment variables are allowed")
	}
	total := 0
	for name, value := range env {
		if !environmentName.MatchString(name) {
			return fmt.Errorf("invalid environment variable name: %s", name)
		}
		if name == "PORT" || name == "REDIS_URL" {
			return fmt.Errorf("environment variable %s is managed by Atlas", name)
		}
		if strings.ContainsRune(value, 0) {
			return fmt.Errorf("environment variable %s contains a null byte", name)
		}
		total += len(name) + len(value)
	}
	if total > 65536 {
		return fmt.Errorf("environment variables must total at most 64 KiB")
	}
	return nil
}
