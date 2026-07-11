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

	addr := os.Getenv("ATLAS_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	if err := api.New(addr, st).Run(); err != nil {
		log.Fatal(err)
	}
}
