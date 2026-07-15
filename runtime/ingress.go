package runtime

import (
	"context"
	"fmt"

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressOptions configures an Ingress for an app.
type IngressOptions struct {
	Namespace        string
	Name             string
	Host             string
	Port             int32
	IngressClassName string
	TLSSecretName    string
	Labels           map[string]string
	ProjectID        string
	ProjectName      string
}

// EnsureIngress creates or updates an Ingress routing host to the app's Service.
func (c *Client) EnsureIngress(ctx context.Context, opts IngressOptions) error {
	if opts.Name == "" {
		return fmt.Errorf("ingress: name is required")
	}
	if opts.Host == "" {
		return fmt.Errorf("ingress: host is required")
	}
	if opts.Namespace == "" {
		opts.Namespace = "default"
	}
	if opts.Port == 0 {
		opts.Port = defaultContainerPort
	}

	ing := desiredIngress(opts)
	ingresses := c.clientset.NetworkingV1().Ingresses(opts.Namespace)

	_, err := ingresses.Create(ctx, ing, metav1.CreateOptions{})
	if err == nil {
		return nil
	}
	if !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create ingress: %w", err)
	}

	existing, err := ingresses.Get(ctx, opts.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get ingress: %w", err)
	}

	existing.Labels = MergeLabels(existing.Labels, desiredIngress(opts).Labels)
	existing.Spec.IngressClassName = ingressClassName(opts.IngressClassName)
	existing.Spec.Rules = ingressRules(opts)
	existing.Spec.TLS = ingressTLS(opts)
	_, err = ingresses.Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("update ingress: %w", err)
	}
	return nil
}

func desiredIngress(opts IngressOptions) *networkingv1.Ingress {
	labels := ProjectLabels(opts.ProjectID, opts.ProjectName)
	labels["app"] = opts.Name
	labels = MergeLabels(labels, opts.Labels)

	className := ingressClassName(opts.IngressClassName)
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:   opts.Name,
			Labels: labels,
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: className,
			Rules:            ingressRules(opts),
			TLS:              ingressTLS(opts),
		},
	}
}

func ingressRules(opts IngressOptions) []networkingv1.IngressRule {
	pathType := networkingv1.PathTypePrefix
	return []networkingv1.IngressRule{
		{
			Host: opts.Host,
			IngressRuleValue: networkingv1.IngressRuleValue{
				HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						{
							Path:     "/",
							PathType: &pathType,
							Backend: networkingv1.IngressBackend{
								Service: &networkingv1.IngressServiceBackend{
									Name: opts.Name,
									Port: networkingv1.ServiceBackendPort{
										Number: opts.Port,
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func ingressClassName(name string) *string {
	if name == "" {
		return nil
	}
	return &name
}

func ingressTLS(opts IngressOptions) []networkingv1.IngressTLS {
	if opts.TLSSecretName == "" {
		return nil
	}
	return []networkingv1.IngressTLS{
		{
			Hosts:      []string{opts.Host},
			SecretName: opts.TLSSecretName,
		},
	}
}
