package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/meshestra/go-playground/jp/internal/auth"
	"github.com/meshestra/go-playground/jp/internal/config"
	"github.com/meshestra/go-playground/jp/internal/src-agent"
	"github.com/meshestra/go-playground/jp/internal/src-agent/memory"
	"github.com/meshestra/go-playground/jp/internal/src-api/presentation/http/adapter"
	"github.com/meshestra/go-playground/jp/internal/src-api/presentation/http/controller"
	"github.com/meshestra/go-playground/jp/internal/src-api/presentation/http/router"
)

func bootAPI(ctx context.Context, cfg config.Config, store memory.Store, dbClose func() error) error {
	runtime, err := agent.NewConfiguredRuntimeWithStore(cfg, store)
	if err != nil {
		return err
	}
	defer runtime.Close()
	if dbClose != nil {
		defer dbClose()
	}
	authService, err := auth.NewService(runtime, auth.Config{Secret: cfg.JWTSecret, Issuer: cfg.JWTIssuer, Audience: cfg.JWTAudience})
	if err != nil {
		return err
	}
	generator, err := agent.New()
	if err != nil {
		return err
	}
	c, err := controller.NewController(cfg.ContentDir, adapter.Dependencies{Chat: runtime, Auth: authService, Generator: generator})
	if err != nil {
		return err
	}
	handler, err := router.New(c)
	if err != nil {
		return fmt.Errorf("create API router: %w", err)
	}
	server := &http.Server{Addr: cfg.WebAddr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}
	serverError := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverError <- err
		}
		close(serverError)
	}()
	select {
	case err := <-serverError:
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdown)
	}
}
