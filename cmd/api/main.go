package main

import (
	"context"
	"log"
	"os"

	"github.com/pixll/atlas/api"
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

	if err := api.New(addr, st, webhookSecret, registry).Run(); err != nil {
		log.Fatal(err)
	}
}
