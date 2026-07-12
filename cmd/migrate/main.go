package main

import (
	"context"
	"github.com/hubvas/internal/infrastructure/persistence/postgres"
	"github.com/hubvas/pkg/config"
	"log"
	"os"
)

func main() {
	path := os.Getenv("HUBVAS_CONFIG")
	if path == "" {
		path = "configs/config.yaml"
	}
	cfg, err := config.Load(path)
	if err != nil {
		log.Fatal(err)
	}
	pool, err := postgres.NewPool(context.Background(), cfg.Database)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	if err = postgres.Migrate(context.Background(), pool); err != nil {
		log.Fatal(err)
	}
	log.Println("database migrations are up to date")
}
