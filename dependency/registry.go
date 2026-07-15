package dependency

import (
	"fmt"

	"github.com/pixll/atlas/config"
)

// Registry maps dependency types to provisioners.
type Registry struct {
	provisioners map[config.DependencyType]Provisioner
}

// NewRegistry returns an empty provisioner registry.
func NewRegistry() *Registry {
	return &Registry{provisioners: make(map[config.DependencyType]Provisioner)}
}

// Register associates a dependency type with a provisioner.
func (r *Registry) Register(t config.DependencyType, p Provisioner) {
	r.provisioners[t] = p
}

// Get returns the provisioner for a dependency type.
func (r *Registry) Get(t config.DependencyType) (Provisioner, error) {
	p, ok := r.provisioners[t]
	if !ok {
		return nil, fmt.Errorf("no provisioner registered for dependency type %q", t)
	}
	return p, nil
}
