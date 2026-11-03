package main

import (
	"context"
	"log"
	"os"
	"os/signal"

	"github.com/meshestra/go-playground/jp/internal/config"
	"github.com/meshestra/go-playground/jp/internal/src-agent"
	"github.com/meshestra/go-playground/jp/internal/src-batch"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	generator, err := agent.New()
	if err != nil {
		log.Fatal(err)
	}
	mailer := scheduler.NewSMTPMailer(cfg)
	job, err := scheduler.New(cfg, generator.Run, mailer, nil)
	if err != nil {
		log.Fatal(err)
	}
	job.SetIndexer(generator.IndexDaily)
	log.Printf("batch scheduler enabled at 06:00 (%s)", cfg.ScheduleTZ)
	if err := job.Run(ctx); err != nil && err != context.Canceled {
		log.Fatal(err)
	}
}
