package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/meshestra/go-playground/jp/internal/auth"
	"github.com/meshestra/go-playground/jp/internal/config"
	"github.com/meshestra/go-playground/jp/internal/src-agent"
	"github.com/meshestra/go-playground/jp/internal/src-agent/memory"
	"github.com/meshestra/go-playground/jp/internal/src-api/infrastructure/postgres"
	httpapi "github.com/meshestra/go-playground/jp/internal/src-api/presentation/http"
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
		store = postgresStore
		dbClose = db.Close
	}
	runtime, err := agent.NewConfiguredRuntimeWithStore(cfg, store)
	if err != nil {
		log.Fatal(err)
	}
	defer runtime.Close()
	if dbClose != nil {
		defer dbClose()
	}
	authService, err := auth.NewService(runtime, auth.Config{Secret: cfg.JWTSecret, Issuer: cfg.JWTIssuer, Audience: cfg.JWTAudience})
	if err != nil {
		log.Fatal(err)
	}
	generator, err := agent.New()
	if err != nil {
		log.Fatal(err)
	}
	server, err := httpapi.NewServerWithAuthAndGenerator(cfg.WebAddr, cfg.ContentDir, runtime, authService, generator)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("api server listening on %s", cfg.WebAddr)
	if err := server.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
