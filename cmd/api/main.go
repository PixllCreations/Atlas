package main

import (
	"context"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/pixll/atlas/api"
	"github.com/pixll/atlas/build"
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

	workerCfg := build.WorkerConfig{
		Registry:           os.Getenv("ATLAS_REGISTRY_URL"),
		Namespace:          os.Getenv("ATLAS_K8S_NAMESPACE"),
		IngressDomain:      os.Getenv("ATLAS_INGRESS_DOMAIN"),
		IngressClass:       os.Getenv("ATLAS_INGRESS_CLASS"),
		IngressTLSSecret:   os.Getenv("ATLAS_INGRESS_TLS_SECRET"),
		RegistrySecretName: os.Getenv("ATLAS_REGISTRY_SECRET"),
		InsecureRegistry:   envBool("ATLAS_INSECURE_REGISTRY"),
	}

	var deployer build.Deployer
	if rt, err := runtime.New(os.Getenv("ATLAS_KUBECONFIG")); err != nil {
		log.Printf("kubernetes unavailable, deploys disabled: %v", err)
	} else {
		deployer = rt
	}

	if err := api.New(addr, st, webhookSecret, workerCfg, deployer).Run(); err != nil {
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
