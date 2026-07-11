package main

import (
	"context"
	"log"
	"os"

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
	registry := os.Getenv("ATLAS_REGISTRY_URL")
	namespace := os.Getenv("ATLAS_K8S_NAMESPACE")

	var deployer build.Deployer
	if rt, err := runtime.New(os.Getenv("ATLAS_KUBECONFIG")); err != nil {
		log.Printf("kubernetes unavailable, deploys disabled: %v", err)
	} else {
		deployer = rt
	}

	if err := api.New(addr, st, webhookSecret, registry, deployer, namespace).Run(); err != nil {
		log.Fatal(err)
	}
}
