package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/meshestra/go-playground/jp/internal/auth"
	"github.com/meshestra/go-playground/jp/internal/llm"
)

type LearningProfile struct {
	Level            string   `json:"level"`
	Goals            []string `json:"goals"`
	WeakGrammar      []string `json:"weak_grammar"`
	KnownVocabulary  []string `json:"known_vocabulary"`
	RepeatedMistakes []string `json:"repeated_mistakes"`
	PreferredStyle   string   `json:"preferred_style"`
}

type Episode struct {
	SessionID string  `json:"session_id"`
	Topic     string  `json:"topic"`
	Score     float64 `json:"score"`
	Summary   string  `json:"summary"`
}

type LearningState struct {
	LessonID   string    `json:"lesson_id"`
	Viewed     bool      `json:"viewed"`
	Practiced  bool      `json:"practiced"`
	Correct    int       `json:"correct"`
	Attempts   int       `json:"attempts"`
	Mastered   bool      `json:"mastered"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

type Store interface {
	LoadMessages(ctx context.Context, userID, sessionID string, limit int) ([]llm.Message, error)
	AppendMessage(ctx context.Context, userID, sessionID string, message llm.Message) error
	GetLearningProfile(ctx context.Context, userID string) (LearningProfile, error)
	SaveLearningProfile(ctx context.Context, userID string, profile LearningProfile) error
	AddEpisode(ctx context.Context, userID string, episode Episode) error
	ListEpisodes(ctx context.Context, userID string, limit int) ([]Episode, error)
	GetLearningState(ctx context.Context, userID, lessonID string) (LearningState, error)
	SaveLearningState(ctx context.Context, userID string, state LearningState) error
	CreateUser(context.Context, auth.User) error
	GetUserByEmail(context.Context, string) (auth.User, error)
	GetUser(context.Context, string) (auth.User, error)
	SaveTopicSetting(context.Context, auth.TopicSetting) error
	GetTopicSetting(context.Context, string, string) (auth.TopicSetting, error)
	ListTopicSettings(context.Context, string) ([]auth.TopicSetting, error)
}

type InMemoryStore struct {
	mu       sync.RWMutex
	messages map[string][]llm.Message
	profiles map[string]LearningProfile
	episodes map[string][]Episode
	states   map[string]LearningState
	users    map[string]auth.User
	emails   map[string]string
	topics   map[string]auth.TopicSetting
}

func NewInMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		messages: make(map[string][]llm.Message),
		profiles: make(map[string]LearningProfile),
		episodes: make(map[string][]Episode),
		states:   make(map[string]LearningState),
		users:    make(map[string]auth.User),
		emails:   make(map[string]string),
		topics:   make(map[string]auth.TopicSetting),
	}
}

func (s *InMemoryStore) CreateUser(ctx context.Context, user auth.User) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.emails[user.Email]; exists {
		return auth.ErrEmailExists
	}
	s.users[user.ID] = user
	s.emails[user.Email] = user.ID
	return nil
}

func (s *InMemoryStore) GetUserByEmail(ctx context.Context, email string) (auth.User, error) {
	if err := contextError(ctx); err != nil {
		return auth.User{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.emails[email]
	if !ok {
		return auth.User{}, fmt.Errorf("user not found")
	}
	return s.users[id], nil
}

func (s *InMemoryStore) GetUser(ctx context.Context, id string) (auth.User, error) {
	if err := contextError(ctx); err != nil {
		return auth.User{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	user, ok := s.users[id]
	if !ok {
		return auth.User{}, fmt.Errorf("user not found")
	}
	return user, nil
}

func (s *InMemoryStore) SaveTopicSetting(ctx context.Context, setting auth.TopicSetting) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	setting.UpdatedAt = time.Now().UTC()
	s.topics[setting.UserID+"\x00"+setting.ApplyDate] = setting
	return nil
}

func (s *InMemoryStore) GetTopicSetting(ctx context.Context, userID, date string) (auth.TopicSetting, error) {
	if err := contextError(ctx); err != nil {
		return auth.TopicSetting{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.topics[userID+"\x00"+date], nil
}

func (s *InMemoryStore) ListTopicSettings(ctx context.Context, date string) ([]auth.TopicSetting, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	var result []auth.TopicSetting
	for _, setting := range s.topics {
		if setting.ApplyDate == date {
			result = append(result, setting)
		}
	}
	return result, nil
}

func (s *InMemoryStore) GetLearningState(ctx context.Context, userID, lessonID string) (LearningState, error) {
	if err := contextError(ctx); err != nil {
		return LearningState{}, err
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.states[userID+"\x00"+lessonID], nil
}

func (s *InMemoryStore) SaveLearningState(ctx context.Context, userID string, state LearningState) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if userID == "" || state.LessonID == "" {
		return fmt.Errorf("user ID and lesson ID are required")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if state.LastSeenAt.IsZero() {
		state.LastSeenAt = time.Now()
	}
	s.states[userID+"\x00"+state.LessonID] = state
	return nil
}

func (s *InMemoryStore) LoadMessages(ctx context.Context, userID, sessionID string, limit int) ([]llm.Message, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	if limit < 0 {
		return nil, fmt.Errorf("message limit cannot be negative")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	messages := s.messages[sessionKey(userID, sessionID)]
	start := 0
	if limit > 0 && len(messages) > limit {
		start = len(messages) - limit
	}

	return append([]llm.Message(nil), messages[start:]...), nil
}

func (s *InMemoryStore) AppendMessage(ctx context.Context, userID, sessionID string, message llm.Message) error {
	if err := contextError(ctx); err != nil {
		return err
	}

	if userID == "" || sessionID == "" {
		return fmt.Errorf("user ID and session ID are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	key := sessionKey(userID, sessionID)
	s.messages[key] = append(s.messages[key], message)

	return nil
}

func (s *InMemoryStore) GetLearningProfile(ctx context.Context, userID string) (LearningProfile, error) {
	if err := contextError(ctx); err != nil {
		return LearningProfile{}, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	return cloneProfile(s.profiles[userID]), nil
}

func (s *InMemoryStore) SaveLearningProfile(ctx context.Context, userID string, profile LearningProfile) error {
	if err := contextError(ctx); err != nil {
		return err
	}

	if userID == "" {
		return fmt.Errorf("user ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.profiles[userID] = cloneProfile(profile)

	return nil
}

func (s *InMemoryStore) AddEpisode(ctx context.Context, userID string, episode Episode) error {
	if err := contextError(ctx); err != nil {
		return err
	}

	if userID == "" {
		return fmt.Errorf("user ID is required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.episodes[userID] = append(s.episodes[userID], episode)

	return nil
}

func (s *InMemoryStore) ListEpisodes(ctx context.Context, userID string, limit int) ([]Episode, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}

	if limit < 0 {
		return nil, fmt.Errorf("episode limit cannot be negative")
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	episodes := s.episodes[userID]
	start := 0
	if limit > 0 && len(episodes) > limit {
		start = len(episodes) - limit
	}

	return append([]Episode(nil), episodes[start:]...), nil
}

func sessionKey(userID, sessionID string) string {
	return userID + "\x00" + sessionID
}

func contextError(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func cloneProfile(profile LearningProfile) LearningProfile {
	profile.Goals = append([]string(nil), profile.Goals...)
	profile.WeakGrammar = append([]string(nil), profile.WeakGrammar...)
	profile.KnownVocabulary = append([]string(nil), profile.KnownVocabulary...)
	profile.RepeatedMistakes = append([]string(nil), profile.RepeatedMistakes...)

	return profile
}
