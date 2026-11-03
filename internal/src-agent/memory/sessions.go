package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// SessionSummary is the data needed by a chat-history sidebar. It intentionally
// contains no full transcript; transcripts remain loaded only for the active
// session.
type SessionSummary struct {
	SessionID   string    `json:"session_id"`
	Title       string    `json:"title"`
	LastMessage string    `json:"last_message"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// SessionLister is optional so existing memory.Store implementations remain
// compatible while clients can progressively add chat-history navigation.
type SessionLister interface {
	ListSessions(context.Context, string, int) ([]SessionSummary, error)
}

func (s *InMemoryStore) ListSessions(ctx context.Context, userID string, limit int) ([]SessionSummary, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if limit < 0 {
		return nil, fmt.Errorf("session limit cannot be negative")
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]SessionSummary, 0)
	prefix := userID + "\x00"
	for key, messages := range s.messages {
		if !strings.HasPrefix(key, prefix) || len(messages) == 0 {
			continue
		}
		sessionID := strings.TrimPrefix(key, prefix)
		last := messages[len(messages)-1]
		title := strings.TrimSpace(last.Content)
		if len([]rune(title)) > 48 {
			title = string([]rune(title)[:48]) + "…"
		}
		result = append(result, SessionSummary{SessionID: sessionID, Title: title, LastMessage: last.Content})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].SessionID < result[j].SessionID })
	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result, nil
}
