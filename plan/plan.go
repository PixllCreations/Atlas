package plan

import (
	"fmt"
	"sort"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	"github.com/pixll/atlas/config"
)

const ApplicationName = "app"

// DeploymentPlan is the desired Kubernetes state derived from atlas.yaml.
type DeploymentPlan struct {
	ProjectID   string
	ProjectName string
	Namespace   string
	Host        string

	Application  ApplicationPlan
	Dependencies []DependencyPlan
}

// ApplicationPlan describes the primary application workload.
type ApplicationPlan struct {
	Name  string
	Image string
	Port  int32
	Env   []corev1.EnvVar
}

// DependencyPlan describes a managed dependency to provision.
type DependencyPlan struct {
	Name   string
	Type   config.DependencyType
	Config config.Dependency
}

// BuildOptions are inputs for constructing a DeploymentPlan.
type BuildOptions struct {
	ProjectID     string
	ProjectName   string
	Image         string
	IngressDomain string
	Config        config.Config
}

// NamespaceName returns the Atlas-managed namespace for a project.
func NamespaceName(projectName string) string {
	return "atlas-" + projectName
}

// Build converts project metadata + atlas config + image into a DeploymentPlan.
func Build(opts BuildOptions) (DeploymentPlan, error) {
	if opts.ProjectName == "" {
		return DeploymentPlan{}, fmt.Errorf("project name is required")
	}
	if opts.Image == "" {
		return DeploymentPlan{}, fmt.Errorf("image is required")
	}

	ns := NamespaceName(opts.ProjectName)
	port := opts.Config.App.Port

	deps := make([]DependencyPlan, 0, len(opts.Config.Dependencies))
	envMap := map[string]string{
		"PORT": strconv.Itoa(int(port)),
	}

	names := make([]string, 0, len(opts.Config.Dependencies))
	for name := range opts.Config.Dependencies {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		dep := opts.Config.Dependencies[name]
		deps = append(deps, DependencyPlan{
			Name:   name,
			Type:   dep.Type,
			Config: dep,
		})
		switch dep.Type {
		case config.DependencyRedis:
			envMap["REDIS_URL"] = fmt.Sprintf("redis://%s:6379", name)
		}
	}

	envKeys := make([]string, 0, len(envMap))
	for k := range envMap {
		envKeys = append(envKeys, k)
	}
	sort.Strings(envKeys)

	env := make([]corev1.EnvVar, 0, len(envKeys))
	for _, k := range envKeys {
		env = append(env, corev1.EnvVar{Name: k, Value: envMap[k]})
	}

	var host string
	if opts.IngressDomain != "" {
		host = opts.ProjectName + "." + opts.IngressDomain
	}

	return DeploymentPlan{
		ProjectID:   opts.ProjectID,
		ProjectName: opts.ProjectName,
		Namespace:   ns,
		Host:        host,
		Application: ApplicationPlan{
			Name:  ApplicationName,
			Image: opts.Image,
			Port:  port,
			Env:   env,
		},
		Dependencies: deps,
	}, nil
}
