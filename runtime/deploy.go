package runtime

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeployOptions configures a Deployment for an app.
type DeployOptions struct {
	Namespace string
	Name      string
	Image     string
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

	existing.Spec.Template.Spec.Containers[0].Image = opts.Image
	_, err = apps.Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update deployment: %w", err)
	}
	return nil
}

func desiredDeployment(opts DeployOptions) *appsv1.Deployment {
	labels := map[string]string{
		"app":                         opts.Name,
		"app.kubernetes.io/managed-by": "atlas",
	}

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
