package memory

import (
	"context"
	"testing"

	"github.com/meshestra/go-playground/jp/internal/llm"
)

func TestInMemoryStoreListsUserSessions(t *testing.T) {
	store := NewInMemoryStore()
	if err := store.AppendMessage(context.Background(), "user-1", "session-a", llm.Message{Role: llm.RoleUser, Content: "운동 단어"}); err != nil {
		t.Fatal(err)
	}

	if err := store.AppendMessage(context.Background(), "user-2", "session-b", llm.Message{Role: llm.RoleUser, Content: "다른 사용자"}); err != nil {
		t.Fatal(err)
	}

	result, err := store.ListSessions(context.Background(), "user-1", 10)
	if err != nil {
		t.Fatal(err)
	}

	if len(result) != 1 || result[0].SessionID != "session-a" || result[0].Title != "운동 단어" {
		t.Fatalf("unexpected session summaries: %+v", result)
	}
}
