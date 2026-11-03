package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/meshestra/go-playground/jp/internal/agent"
	"github.com/meshestra/go-playground/jp/internal/api"
	"github.com/meshestra/go-playground/jp/internal/auth"
	"github.com/meshestra/go-playground/jp/internal/config"
	"github.com/meshestra/go-playground/jp/internal/repository/postgres"
	"github.com/meshestra/go-playground/jp/internal/scheduler"
)

func main() {
	mode := flag.String("mode", "generate", "generate, agent, web, or scheduler")
	topic := flag.String("topic", "운동과 관련된 일본어", "daily Japanese topic")
	userID := flag.String("user-id", "demo-user", "agent user ID")
	sessionID := flag.String("session-id", "default", "agent session ID")
	input := flag.String("input", "오늘 공부할 일본어를 추천해줘", "agent input")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	switch *mode {
	case "web":
		if err := runWeb(ctx); err != nil {
			log.Fatal(err)
		}
	case "agent":
		if err := runAgent(ctx, *userID, *sessionID, *input); err != nil {
			log.Fatal(err)
		}
	case "scheduler":
		if err := runScheduler(ctx); err != nil && err != context.Canceled {
			log.Fatal(err)
		}
	default:
		if err := generate(*topic); err != nil {
			log.Fatal(err)
		}
	}
}

func runScheduler(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	generator, err := agent.New()
	if err != nil {
		return err
	}
	mailer := scheduler.NewSMTPMailer(cfg)
	job, err := scheduler.New(cfg, generator.Run, mailer, nil)
	if err != nil {
		return err
	}
	job.SetIndexer(generator.IndexDaily)
	if cfg.DatabaseURL != "" {
		db, err := postgres.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		store, err := postgres.NewStore(db)
		if err != nil {
			_ = db.Close()
			return err
		}
		if err := store.EnsureSchema(ctx); err != nil {
			_ = db.Close()
			return err
		}
		job.SetTopicProvider(store.ListTopicSettings)
		job.SetUserGenerator(generator.RunForUser)
		defer db.Close()
	}
	log.Printf("daily scheduler enabled at 06:00 (%s)", cfg.ScheduleTZ)
	return job.Run(ctx)
}

func runAgent(ctx context.Context, userID, sessionID, input string) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	runtime, err := agent.NewConfiguredRuntime(cfg)
	if err != nil {
		return err
	}
	defer runtime.Close()

	response, err := runtime.Run(ctx, userID, sessionID, input)
	if err != nil {
		return err
	}

	fmt.Println(response.Content)

	return nil
}

func generate(topic string) error {

	a, err := agent.New()
	if err != nil {
		return err
	}

	res, err := a.Run(context.Background(), topic)
	if err != nil {
		return err
	}

	fmt.Printf("generated %s: %s\n", res.Date, res.Theme)

	return nil
}

func runWeb(ctx context.Context) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	runtime, err := agent.NewConfiguredRuntime(cfg)
	if err != nil {
		return err
	}
	defer runtime.Close()
	authService, err := auth.NewService(runtime, auth.Config{Secret: cfg.JWTSecret, Issuer: cfg.JWTIssuer, Audience: cfg.JWTAudience})
	if err != nil {
		return err
	}
	generator, err := agent.New()
	if err != nil {
		return err
	}
	server, err := api.NewServerWithAuthAndGenerator(cfg.WebAddr, cfg.ContentDir, runtime, authService, generator)
	if err != nil {
		return err
	}

	log.Printf("web server listening on %s", cfg.WebAddr)

	return server.Run(ctx)
}
