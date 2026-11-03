package memory

import (
	"context"
	"testing"

	"github.com/meshestra/go-playground/jp/internal/src-agent/llm"
)

func TestInMemoryStoreIsolatesUsers(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()

	if err := store.AppendMessage(ctx, "user-1", "session-1", llm.Message{Role: llm.RoleUser, Content: "one"}); err != nil {
		t.Fatalf("append user 1 message: %v", err)
	}
	if err := store.AppendMessage(ctx, "user-2", "session-1", llm.Message{Role: llm.RoleUser, Content: "two"}); err != nil {
		t.Fatalf("append user 2 message: %v", err)
	}

	userOne, err := store.LoadMessages(ctx, "user-1", "session-1", 0)
	if err != nil {
		t.Fatalf("load user 1 messages: %v", err)
	}

	userTwo, err := store.LoadMessages(ctx, "user-2", "session-1", 0)
	if err != nil {
		t.Fatalf("load user 2 messages: %v", err)
	}

	if len(userOne) != 1 || userOne[0].Content != "one" {
		t.Fatalf("unexpected user 1 messages: %+v", userOne)
	}
	if len(userTwo) != 1 || userTwo[0].Content != "two" {
		t.Fatalf("unexpected user 2 messages: %+v", userTwo)
	}
}

func TestInMemoryStoreReturnsRecentMessages(t *testing.T) {
	store := NewInMemoryStore()
	ctx := context.Background()
	for _, content := range []string{"one", "two", "three"} {
		if err := store.AppendMessage(ctx, "user-1", "session-1", llm.Message{Role: llm.RoleUser, Content: content}); err != nil {
			t.Fatalf("append message: %v", err)
		}
	}

	messages, err := store.LoadMessages(ctx, "user-1", "session-1", 2)
	if err != nil {
		t.Fatalf("load messages: %v", err)
	}

	if len(messages) != 2 || messages[0].Content != "two" || messages[1].Content != "three" {
		t.Fatalf("unexpected recent messages: %+v", messages)
	}
}
