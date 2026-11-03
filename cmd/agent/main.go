package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/meshestra/go-playground/jp/internal/config"
	"github.com/meshestra/go-playground/jp/internal/src-agent"
)

func main() {
	topic := flag.String("topic", "운동과 관련된 일본어", "daily Japanese topic")
	userID := flag.String("user-id", "demo-user", "agent user ID")
	sessionID := flag.String("session-id", "default", "agent session ID")
	input := flag.String("input", "", "one-shot agent input; omit to generate daily content")
	flag.Parse()

	ctx := context.Background()
	if *input == "" {
		generator, err := agent.New()
		if err != nil {
			log.Fatal(err)
		}
		result, err := generator.Run(ctx, *topic)
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("generated %s: %s\n", result.Date, result.Theme)
		return
	}
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	runtime, err := agent.NewConfiguredRuntime(cfg)
	if err != nil {
		log.Fatal(err)
	}
	defer runtime.Close()
	response, err := runtime.Run(ctx, *userID, *sessionID, *input)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(response.Content)
}
