package runtime

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ServiceOptions configures a ClusterIP Service for an app.
type ServiceOptions struct {
	Namespace string
	Name      string
	Port      int32
}

// EnsureService creates or updates a ClusterIP Service that selects pods by app label.
func (c *Client) EnsureService(ctx context.Context, opts ServiceOptions) error {
	if opts.Name == "" {
		return fmt.Errorf("service: name is required")
	}
	if opts.Namespace == "" {
		opts.Namespace = "default"
	}
	if opts.Port == 0 {
		opts.Port = 80
	}

	svc := desiredService(opts)
	services := c.clientset.CoreV1().Services(opts.Namespace)

	_, err := services.Create(ctx, svc, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create service: %w", err)
	}

	existing, err := services.Get(ctx, opts.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get service: %w", err)
	}

	existing.Spec.Selector = map[string]string{"app": opts.Name}
	existing.Spec.Ports = []corev1.ServicePort{
		{
			Name:       "http",
			Port:       opts.Port,
			TargetPort: intstr.FromInt32(opts.Port),
			Protocol:   corev1.ProtocolTCP,
		},
	}
	_, err = services.Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update service: %w", err)
	}
	return nil
}

func desiredService(opts ServiceOptions) *corev1.Service {
	labels := map[string]string{
		"app":                          opts.Name,
		"app.kubernetes.io/managed-by": "atlas",
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:   opts.Name,
			Labels: labels,
		},
		Spec: corev1.ServiceSpec{
			Type:     corev1.ServiceTypeClusterIP,
			Selector: map[string]string{"app": opts.Name},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       opts.Port,
					TargetPort: intstr.FromInt32(opts.Port),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}
