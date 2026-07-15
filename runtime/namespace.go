package runtime

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NamespaceOptions configures an Atlas-managed project namespace.
type NamespaceOptions struct {
	Name        string
	ProjectID   string
	ProjectName string
}

// EnsureNamespace creates the namespace if it does not exist.
func (c *Client) EnsureNamespace(ctx context.Context, opts NamespaceOptions) error {
	if opts.Name == "" {
		return fmt.Errorf("namespace: name is required")
	}

	labels := ProjectLabels(opts.ProjectID, opts.ProjectName)
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   opts.Name,
			Labels: labels,
		},
	}

	_, err := c.clientset.CoreV1().Namespaces().Create(ctx, ns, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if apierrors.IsAlreadyExists(err) {
		return nil
	}
	return fmt.Errorf("create namespace: %w", err)
}

// DeleteNamespace deletes a namespace. Missing namespaces are ignored.
func (c *Client) DeleteNamespace(ctx context.Context, name string) error {
	if name == "" {
		return fmt.Errorf("delete namespace: name is required")
	}

	err := c.clientset.CoreV1().Namespaces().Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete namespace: %w", err)
	}
	return nil
}
