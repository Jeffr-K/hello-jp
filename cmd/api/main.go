package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/meshestra/go-playground/jp/internal/config"
	"github.com/meshestra/go-playground/jp/internal/src-agent/memory"
	"github.com/meshestra/go-playground/jp/internal/src-api/infrastructure/postgres"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	var store memory.Store = memory.NewInMemoryStore()
	var dbClose func() error
	if cfg.DatabaseURL != "" {
		db, err := postgres.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			log.Fatal(err)
		}
		postgresStore, err := postgres.NewStore(db)
		if err != nil {
			_ = db.Close()
			log.Fatal(err)
		}
		if err := postgresStore.EnsureSchema(ctx); err != nil {
			_ = db.Close()
			log.Fatal(err)
		}
		store, dbClose = postgresStore, db.Close
	}
	if err := bootAPI(ctx, cfg, store, dbClose); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
