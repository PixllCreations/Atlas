package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// ErrMissing is returned when atlas.yaml is not present in the repository root.
var ErrMissing = errors.New("atlas.yaml not found; add an atlas.yaml to the repository root")

// Load reads and validates atlas.yaml from path.
func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Config{}, ErrMissing
		}
		return Config{}, fmt.Errorf("read atlas.yaml: %w", err)
	}
	return Parse(data)
}

// Parse unmarshals and validates atlas.yaml content.
func Parse(data []byte) (Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parse atlas.yaml: %w", err)
	}
	if err := Validate(cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}
