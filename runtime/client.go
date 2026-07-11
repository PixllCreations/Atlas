package runtime

import (
	"fmt"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Client wraps a Kubernetes clientset for Atlas runtime operations.
type Client struct {
	clientset *kubernetes.Clientset
}

// New connects to a Kubernetes cluster using kubeconfig when set.
// An empty kubeconfig uses in-cluster config, then the default local kubeconfig.
func New(kubeconfig string) (*Client, error) {
	cfg, err := loadRESTConfig(kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("load kubeconfig: %w", err)
	}

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}

	if _, err := cs.Discovery().ServerVersion(); err != nil {
		return nil, fmt.Errorf("connect kubernetes: %w", err)
	}

	return &Client{clientset: cs}, nil
}

func loadRESTConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig != "" {
		return clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if cfg, err := rest.InClusterConfig(); err == nil {
		return cfg, nil
	}

	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	loader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, &clientcmd.ConfigOverrides{})
	return loader.ClientConfig()
}
