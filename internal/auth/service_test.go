package auth_test

import (
	"context"
	"testing"
	"time"

	"github.com/meshestra/go-playground/jp/internal/auth"
	"github.com/meshestra/go-playground/jp/internal/src-agent/memory"
)

func TestRegisterLoginAndAuthenticate(t *testing.T) {
	service, err := auth.NewService(memory.NewInMemoryStore(), auth.Config{Secret: "test-secret", AccessTTL: time.Hour})
	if err != nil {
		t.Fatal(err)
	}
	user, token, err := service.Register(context.Background(), "User@Example.com", "password123", "학습자")
	if err != nil {
		t.Fatal(err)
	}
	if user.Email != "user@example.com" || token == "" {
		t.Fatalf("unexpected registration result: %+v", user)
	}
	user, token, err = service.Login(context.Background(), "user@example.com", "password123")
	if err != nil {
		t.Fatal(err)
	}
	if user.ID == "" {
		t.Fatal("expected user ID")
	}
	id, err := service.Authenticate("Bearer " + token)
	if err != nil || id != user.ID {
		t.Fatalf("authenticate: id=%q err=%v", id, err)
	}
}

func TestTopicSetting(t *testing.T) {
	service, err := auth.NewService(memory.NewInMemoryStore(), auth.Config{Secret: "test-secret"})
	if err != nil {
		t.Fatal(err)
	}
	user, _, err := service.Register(context.Background(), "topic@example.com", "password123", "학습자")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.SaveTopic(context.Background(), auth.TopicSetting{UserID: user.ID, Topic: "일본 여행 회화", ApplyDate: "2026-11-04"}); err != nil {
		t.Fatal(err)
	}
	setting, err := service.Topic(context.Background(), user.ID, "2026-11-04")
	if err != nil || setting.Topic != "일본 여행 회화" {
		t.Fatalf("unexpected topic setting: %+v err=%v", setting, err)
	}
}
