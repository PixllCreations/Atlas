package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/pixll/atlas/api"
	"github.com/pixll/atlas/build"
	"github.com/pixll/atlas/github"
	"github.com/pixll/atlas/runtime"
	"github.com/pixll/atlas/store"
)

func main() {
	ctx := context.Background()

	dsn := os.Getenv("ATLAS_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://atlas:atlas@localhost:5432/atlas?sslmode=disable"
	}

	st, err := store.New(ctx, dsn)
	if err != nil {
		log.Fatal(err)
	}
	defer st.Close()

	port := os.Getenv("ATLAS_PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port

	webhookSecret := os.Getenv("ATLAS_WEBHOOK_SECRET")
	webhookPublicURL := os.Getenv("ATLAS_WEBHOOK_PUBLIC_URL")

	ghCfg, err := github.LoadConfig()
	if err != nil {
		log.Fatalf("github app config: %v", err)
	}
	var ghClient *github.Client
	if ghCfg.Enabled() {
		ghClient, err = github.NewClient(ghCfg)
		if err != nil {
			log.Fatalf("github app client: %v", err)
		}
		log.Printf("github app enabled (%s)", ghCfg.AppSlug)
	}

	workerCfg := build.WorkerConfig{
		Registry:           os.Getenv("ATLAS_REGISTRY_URL"),
		Namespace:          os.Getenv("ATLAS_K8S_NAMESPACE"),
		IngressDomain:      os.Getenv("ATLAS_INGRESS_DOMAIN"),
		IngressClass:       os.Getenv("ATLAS_INGRESS_CLASS"),
		IngressTLSSecret:   os.Getenv("ATLAS_INGRESS_TLS_SECRET"),
		RegistrySecretName: os.Getenv("ATLAS_REGISTRY_SECRET"),
		InsecureRegistry:   envBool("ATLAS_INSECURE_REGISTRY"),
	}

	deployer, err := runtime.New(os.Getenv("ATLAS_KUBECONFIG"))
	if err != nil {
		log.Fatalf("kubernetes is required: %v\n"+
			"set ATLAS_KUBECONFIG or place a kubeconfig at ~/.kube/config\n"+
			"if the cluster server is 127.0.0.1, use host.docker.internal or a Tailscale IP (reachable from this process)",
			err)
	}
	log.Printf("kubernetes connected; Job builds and deploys enabled")

	if err := api.New(api.Options{
		Addr:             addr,
		Store:            st,
		WebhookSecret:    webhookSecret,
		GitHub:           ghClient,
		WebhookPublicURL: webhookPublicURL,
		WorkerConfig:     workerCfg,
		Deployer:         deployer,
		Status: api.StatusConfig{
			Port:                port,
			IngressDomain:       workerCfg.IngressDomain,
			RegistryURL:         workerCfg.Registry,
			Namespace:           workerCfg.Namespace,
			KubernetesOK:        true,
			WebhookSecret:       webhookSecret != "",
			WebhookPublicURL:    webhookPublicURL,
			GitHubAppConfigured: ghClient != nil,
			GitHubAppSlug:       ghCfg.AppSlug,
		},
	}).Run(); err != nil {
		log.Fatal(err)
	}
}

func envBool(key string) bool {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		log.Printf("invalid %s=%q, treating as false", key, v)
		return false
	}
	return b
}
