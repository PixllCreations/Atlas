package runtime

import (
	"context"
	"fmt"
	"sort"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const defaultContainerPort int32 = 80

// DeployOptions configures a Deployment for an app or dependency.
type DeployOptions struct {
	Namespace   string
	Name        string
	Image       string
	Port        int32
	Env         []corev1.EnvVar
	Labels      map[string]string
	ProjectID   string
	ProjectName string
}

// EnsureDeployment creates or updates a Deployment to run image.
func (c *Client) EnsureDeployment(ctx context.Context, opts DeployOptions) error {
	if opts.Name == "" {
		return fmt.Errorf("deploy: name is required")
	}
	if opts.Image == "" {
		return fmt.Errorf("deploy: image is required")
	}
	if opts.Namespace == "" {
		opts.Namespace = "default"
	}
	if opts.Port == 0 {
		opts.Port = defaultContainerPort
	}
	opts.Env = NormalizeEnv(opts.Env)

	dep := desiredDeployment(opts)
	apps := c.clientset.AppsV1().Deployments(opts.Namespace)

	_, err := apps.Create(ctx, dep, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create deployment: %w", err)
	}

	existing, err := apps.Get(ctx, opts.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get deployment: %w", err)
	}

	if len(existing.Spec.Template.Spec.Containers) == 0 {
		return fmt.Errorf("update deployment: existing deployment has no containers")
	}

	existing.Labels = MergeLabels(existing.Labels, dep.Labels)
	existing.Spec.Template.Labels = MergeLabels(existing.Spec.Template.Labels, dep.Spec.Template.Labels)
	existing.Spec.Template.Spec.Containers[0].Image = opts.Image
	existing.Spec.Template.Spec.Containers[0].Ports = containerPorts(opts.Port)
	existing.Spec.Template.Spec.Containers[0].Env = opts.Env
	_, err = apps.Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update deployment: %w", err)
	}
	return nil
}

// NormalizeEnv returns a stable sorted copy of env vars by name.
func NormalizeEnv(env []corev1.EnvVar) []corev1.EnvVar {
	if len(env) == 0 {
		return nil
	}
	out := make([]corev1.EnvVar, len(env))
	copy(out, env)
	sort.Slice(out, func(i, j int) bool {
		return out[i].Name < out[j].Name
	})
	return out
}

func desiredDeployment(opts DeployOptions) *appsv1.Deployment {
	labels := ProjectLabels(opts.ProjectID, opts.ProjectName)
	labels["app"] = opts.Name
	labels = MergeLabels(labels, opts.Labels)

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:   opts.Name,
			Labels: labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32Ptr(1),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": opts.Name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            opts.Name,
							Image:           opts.Image,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports:           containerPorts(opts.Port),
							Env:             opts.Env,
						},
					},
				},
			},
		},
	}
}

func int32Ptr(v int32) *int32 {
	return &v
}

func containerPorts(port int32) []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{
			Name:          "http",
			ContainerPort: port,
			Protocol:      corev1.ProtocolTCP,
		},
	}
}

// ListManagedDependencyDeployments lists Atlas-managed dependency Deployments in a namespace.
func (c *Client) ListManagedDependencyDeployments(ctx context.Context, namespace, projectID string) ([]appsv1.Deployment, error) {
	if namespace == "" {
		namespace = "default"
	}
	selector := fmt.Sprintf("%s=%s,%s=%s", LabelManagedBy, LabelManagedByValue, LabelComponent, ComponentDependency)
	if projectID != "" {
		selector += fmt.Sprintf(",%s=%s", LabelProjectID, projectID)
	}
	list, err := c.clientset.AppsV1().Deployments(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("list dependency deployments: %w", err)
	}
	return list.Items, nil
}

// ListManagedDependencyServices lists Atlas-managed dependency Services in a namespace.
func (c *Client) ListManagedDependencyServices(ctx context.Context, namespace, projectID string) ([]corev1.Service, error) {
	if namespace == "" {
		namespace = "default"
	}
	selector := fmt.Sprintf("%s=%s,%s=%s", LabelManagedBy, LabelManagedByValue, LabelComponent, ComponentDependency)
	if projectID != "" {
		selector += fmt.Sprintf(",%s=%s", LabelProjectID, projectID)
	}
	list, err := c.clientset.CoreV1().Services(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, fmt.Errorf("list dependency services: %w", err)
	}
	return list.Items, nil
}
