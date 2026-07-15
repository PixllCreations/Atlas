package runtime

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DeleteDeployment removes an app Deployment. Missing resources are ignored.
func (c *Client) DeleteDeployment(ctx context.Context, namespace, name string) error {
	if name == "" {
		return fmt.Errorf("delete deployment: name is required")
	}
	if namespace == "" {
		namespace = "default"
	}

	err := c.clientset.AppsV1().Deployments(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete deployment: %w", err)
	}
	return nil
}

// DeleteService removes an app Service. Missing resources are ignored.
func (c *Client) DeleteService(ctx context.Context, namespace, name string) error {
	if name == "" {
		return fmt.Errorf("delete service: name is required")
	}
	if namespace == "" {
		namespace = "default"
	}

	err := c.clientset.CoreV1().Services(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete service: %w", err)
	}
	return nil
}

// DeleteIngress removes an app Ingress. Missing resources are ignored.
func (c *Client) DeleteIngress(ctx context.Context, namespace, name string) error {
	if name == "" {
		return fmt.Errorf("delete ingress: name is required")
	}
	if namespace == "" {
		namespace = "default"
	}

	err := c.clientset.NetworkingV1().Ingresses(namespace).Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete ingress: %w", err)
	}
	return nil
}
