package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/meshestra/go-playground/jp/internal/auth"
	"github.com/meshestra/go-playground/jp/internal/src-agent/llm"
	"github.com/meshestra/go-playground/jp/internal/src-agent/memory"
)

type Store struct {
	db *sql.DB
}

func (s *Store) CreateUser(ctx context.Context, user auth.User) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO users (id, email, name, password_hash, created_at) VALUES ($1, $2, $3, $4, $5)`, user.ID, user.Email, user.Name, user.PasswordHash, user.CreatedAt)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (auth.User, error) {
	var user auth.User
	err := s.db.QueryRowContext(ctx, `SELECT id, email, name, password_hash, created_at FROM users WHERE email = $1`, email).Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return auth.User{}, fmt.Errorf("get user by email: %w", err)
	}
	return user, nil
}

func (s *Store) GetUser(ctx context.Context, id string) (auth.User, error) {
	var user auth.User
	err := s.db.QueryRowContext(ctx, `SELECT id, email, name, password_hash, created_at FROM users WHERE id = $1`, id).Scan(&user.ID, &user.Email, &user.Name, &user.PasswordHash, &user.CreatedAt)
	if err != nil {
		return auth.User{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (s *Store) SaveTopicSetting(ctx context.Context, setting auth.TopicSetting) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO topic_settings (user_id, apply_date, topic, updated_at) VALUES ($1, $2, $3, NOW()) ON CONFLICT (user_id, apply_date) DO UPDATE SET topic = EXCLUDED.topic, updated_at = NOW()`, setting.UserID, setting.ApplyDate, setting.Topic)
	if err != nil {
		return fmt.Errorf("save topic setting: %w", err)
	}
	return nil
}

func (s *Store) GetTopicSetting(ctx context.Context, userID, date string) (auth.TopicSetting, error) {
	var setting auth.TopicSetting
	err := s.db.QueryRowContext(ctx, `SELECT user_id, apply_date, topic, updated_at FROM topic_settings WHERE user_id = $1 AND apply_date = $2`, userID, date).Scan(&setting.UserID, &setting.ApplyDate, &setting.Topic, &setting.UpdatedAt)
	if err != nil {
		return auth.TopicSetting{}, fmt.Errorf("get topic setting: %w", err)
	}
	return setting, nil
}

func (s *Store) ListTopicSettings(ctx context.Context, date string) ([]auth.TopicSetting, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT user_id, apply_date, topic, updated_at FROM topic_settings WHERE apply_date = $1 ORDER BY user_id`, date)
	if err != nil {
		return nil, fmt.Errorf("list topic settings: %w", err)
	}
	defer rows.Close()
	var result []auth.TopicSetting
	for rows.Next() {
		var setting auth.TopicSetting
		if err := rows.Scan(&setting.UserID, &setting.ApplyDate, &setting.Topic, &setting.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan topic setting: %w", err)
		}
		result = append(result, setting)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate topic settings: %w", err)
	}
	return result, nil
}

func NewStore(db *sql.DB) (*Store, error) {
	if db == nil {
		return nil, fmt.Errorf("database is required")
	}

	return &Store{db: db}, nil
}

func (s *Store) EnsureSchema(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("create PostgreSQL schema: %w", err)
	}

	return nil
}

func (s *Store) LoadMessages(ctx context.Context, userID, sessionID string, limit int) ([]llm.Message, error) {
	if limit < 0 {
		return nil, fmt.Errorf("message limit cannot be negative")
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT role, content, tool_calls, tool_call_id, name
		FROM agent_messages
		WHERE user_id = $1 AND session_id = $2
		ORDER BY id DESC
		LIMIT $3`, userID, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("query session messages: %w", err)
	}
	defer rows.Close()

	var reversed []llm.Message
	for rows.Next() {
		var message llm.Message
		var role string
		var toolCalls []byte
		var toolCallID, name sql.NullString

		if err := rows.Scan(&role, &message.Content, &toolCalls, &toolCallID, &name); err != nil {
			return nil, fmt.Errorf("scan session message: %w", err)
		}

		message.Role = llm.Role(role)
		message.ToolCallID = toolCallID.String
		message.Name = name.String
		if len(toolCalls) > 0 {
			if err := json.Unmarshal(toolCalls, &message.ToolCalls); err != nil {
				return nil, fmt.Errorf("decode message tool calls: %w", err)
			}
		}

		reversed = append(reversed, message)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session messages: %w", err)
	}

	messages := make([]llm.Message, len(reversed))
	for index := range reversed {
		messages[len(reversed)-index-1] = reversed[index]
	}

	return messages, nil
}

func (s *Store) ListSessions(ctx context.Context, userID string, limit int) ([]memory.SessionSummary, error) {
	if limit < 0 {
		return nil, fmt.Errorf("session limit cannot be negative")
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT session_id, content, created_at
		FROM (
			SELECT DISTINCT ON (session_id) session_id, content, created_at
			FROM agent_messages
			WHERE user_id = $1 AND role = 'user'
			ORDER BY session_id, id DESC
		) latest
		ORDER BY created_at DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query session summaries: %w", err)
	}
	defer rows.Close()
	var result []memory.SessionSummary
	for rows.Next() {
		var summary memory.SessionSummary
		if err := rows.Scan(&summary.SessionID, &summary.Title, &summary.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan session summary: %w", err)
		}
		summary.LastMessage = summary.Title
		result = append(result, summary)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate session summaries: %w", err)
	}
	return result, nil
}

func (s *Store) AppendMessage(ctx context.Context, userID, sessionID string, message llm.Message) error {
	toolCalls, err := json.Marshal(message.ToolCalls)
	if err != nil {
		return fmt.Errorf("encode message tool calls: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO agent_messages (user_id, session_id, role, content, tool_calls, tool_call_id, name)
		VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), NULLIF($7, ''))`,
		userID, sessionID, message.Role, message.Content, toolCalls, message.ToolCallID, message.Name)
	if err != nil {
		return fmt.Errorf("insert session message: %w", err)
	}

	return nil
}

func (s *Store) GetLearningProfile(ctx context.Context, userID string) (memory.LearningProfile, error) {
	var data []byte
	if err := s.db.QueryRowContext(ctx, `SELECT data FROM learning_profiles WHERE user_id = $1`, userID).Scan(&data); err != nil {
		if err == sql.ErrNoRows {
			return memory.LearningProfile{}, nil
		}
		return memory.LearningProfile{}, fmt.Errorf("get learning profile: %w", err)
	}

	var profile memory.LearningProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return memory.LearningProfile{}, fmt.Errorf("decode learning profile: %w", err)
	}

	return profile, nil
}

func (s *Store) SaveLearningProfile(ctx context.Context, userID string, profile memory.LearningProfile) error {
	data, err := json.Marshal(profile)
	if err != nil {
		return fmt.Errorf("encode learning profile: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO learning_profiles (user_id, data)
		VALUES ($1, $2)
		ON CONFLICT (user_id) DO UPDATE SET data = EXCLUDED.data, updated_at = NOW()`, userID, data)
	if err != nil {
		return fmt.Errorf("save learning profile: %w", err)
	}

	return nil
}

func (s *Store) AddEpisode(ctx context.Context, userID string, episode memory.Episode) error {
	data, err := json.Marshal(episode)
	if err != nil {
		return fmt.Errorf("encode learning episode: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `INSERT INTO learning_episodes (user_id, data) VALUES ($1, $2)`, userID, data)
	if err != nil {
		return fmt.Errorf("save learning episode: %w", err)
	}

	return nil
}

func (s *Store) ListEpisodes(ctx context.Context, userID string, limit int) ([]memory.Episode, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT data
		FROM learning_episodes
		WHERE user_id = $1
		ORDER BY id DESC
		LIMIT $2`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("query learning episodes: %w", err)
	}
	defer rows.Close()

	var reversed []memory.Episode
	for rows.Next() {
		var data []byte
		if err := rows.Scan(&data); err != nil {
			return nil, fmt.Errorf("scan learning episode: %w", err)
		}

		var episode memory.Episode
		if err := json.Unmarshal(data, &episode); err != nil {
			return nil, fmt.Errorf("decode learning episode: %w", err)
		}
		reversed = append(reversed, episode)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate learning episodes: %w", err)
	}

	episodes := make([]memory.Episode, len(reversed))
	for index := range reversed {
		episodes[len(reversed)-index-1] = reversed[index]
	}

	return episodes, nil
}

func (s *Store) GetLearningState(ctx context.Context, userID, lessonID string) (memory.LearningState, error) {
	var data []byte
	err := s.db.QueryRowContext(ctx, "SELECT data FROM learning_states WHERE user_id = $1 AND lesson_id = $2", userID, lessonID).Scan(&data)
	if err == sql.ErrNoRows {
		return memory.LearningState{LessonID: lessonID}, nil
	}
	if err != nil {
		return memory.LearningState{}, fmt.Errorf("get learning state: %w", err)
	}
	var state memory.LearningState
	if err := json.Unmarshal(data, &state); err != nil {
		return memory.LearningState{}, fmt.Errorf("decode learning state: %w", err)
	}
	return state, nil
}

func (s *Store) SaveLearningState(ctx context.Context, userID string, state memory.LearningState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("encode learning state: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		"INSERT INTO learning_states (user_id, lesson_id, data) VALUES ($1, $2, $3) "+
			"ON CONFLICT (user_id, lesson_id) DO UPDATE SET data = EXCLUDED.data, updated_at = NOW()",
		userID, state.LessonID, data)
	if err != nil {
		return fmt.Errorf("save learning state: %w", err)
	}
	return nil
}

const schema = `
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS topic_settings (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    apply_date DATE NOT NULL,
    topic TEXT NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, apply_date)
);

CREATE TABLE IF NOT EXISTS agent_messages (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL DEFAULT '',
    tool_calls JSONB NOT NULL DEFAULT '[]'::jsonb,
    tool_call_id TEXT,
    name TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS agent_messages_session_idx
    ON agent_messages (user_id, session_id, id);

CREATE TABLE IF NOT EXISTS learning_profiles (
    user_id TEXT PRIMARY KEY,
    data JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS learning_episodes (
    id BIGSERIAL PRIMARY KEY,
    user_id TEXT NOT NULL,
    data JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS learning_episodes_user_idx
    ON learning_episodes (user_id, id);

CREATE TABLE IF NOT EXISTS learning_states (
    user_id TEXT NOT NULL,
    lesson_id TEXT NOT NULL,
    data JSONB NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (user_id, lesson_id)
);
`
