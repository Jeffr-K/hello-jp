package config

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config contains runtime settings for the Japanese content generator.
type Config struct {
	AppEnv            string
	LLMProvider       string
	LLMModel          string
	OllamaBaseURL     string
	OpenAIAPIKey      string
	ContentDir        string
	WebAddr           string
	DatabaseURL       string
	DailyLevel        string
	TTSProvider       string
	DailyTopic        string
	ScheduleTZ        string
	SMTPHost          string
	SMTPPort          int
	SMTPUsername      string
	SMTPPassword      string
	EmailFrom         string
	EmailRecipients   []string
	QdrantURL         string
	QdrantCollection  string
	EmbeddingProvider string
	EmbeddingModel    string
}

// Load reads .env when present, then reads the process environment. Existing
// process environment variables always take precedence over values in .env.
func Load() (Config, error) {
	if err := loadDotEnv(".env"); err != nil && !errors.Is(err, os.ErrNotExist) {
		return Config{}, fmt.Errorf("load .env: %w", err)
	}

	cfg := Config{
		AppEnv:            getEnv("APP_ENV", "development"),
		LLMProvider:       getEnv("LLM_PROVIDER", "openai"),
		LLMModel:          getEnv("LLM_MODEL", "gpt-5-mini"),
		OllamaBaseURL:     getEnv("OLLAMA_BASE_URL", "http://127.0.0.1:11434"),
		OpenAIAPIKey:      os.Getenv("OPENAI_API_KEY"),
		ContentDir:        getEnv("CONTENT_DIR", "content"),
		WebAddr:           getEnv("WEB_ADDR", ":8080"),
		DatabaseURL:       os.Getenv("DATABASE_URL"),
		DailyLevel:        getEnv("DAILY_LEVEL", "N3"),
		TTSProvider:       getEnv("TTS_PROVIDER", "browser"),
		DailyTopic:        getEnv("DAILY_TOPIC", "운동과 관련된 일본어"),
		ScheduleTZ:        getEnv("SCHEDULE_TZ", "Asia/Seoul"),
		SMTPHost:          os.Getenv("SMTP_HOST"),
		SMTPPort:          envInt("SMTP_PORT", 587),
		SMTPUsername:      os.Getenv("SMTP_USERNAME"),
		SMTPPassword:      os.Getenv("SMTP_PASSWORD"),
		EmailFrom:         getEnv("EMAIL_FROM", os.Getenv("SMTP_USERNAME")),
		EmailRecipients:   envStringArray("EMAIL_RECIPIENTS"),
		QdrantURL:         getEnv("QDRANT_URL", ""),
		QdrantCollection:  getEnv("QDRANT_COLLECTION", "today_japanese"),
		EmbeddingProvider: getEnv("EMBEDDING_PROVIDER", getEnv("LLM_PROVIDER", "openai")),
		EmbeddingModel:    embeddingModel(),
	}

	return cfg, nil
}

func embeddingModel() string {
	if value := strings.TrimSpace(os.Getenv("EMBEDDING_MODEL")); value != "" {
		return value
	}
	if strings.EqualFold(getEnv("EMBEDDING_PROVIDER", getEnv("LLM_PROVIDER", "openai")), "ollama") {
		return "nomic-embed-text"
	}
	return "text-embedding-3-small"
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envStringArray(key string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return nil
	}
	var result []string
	if json.Unmarshal([]byte(value), &result) == nil {
		return result
	}
	parts := strings.Split(value, ",")
	for index := range parts {
		parts[index] = strings.TrimSpace(parts[index])
	}
	return parts
}

func getEnv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}

	return fallback
}

func loadDotEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	defer file.Close()

	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, value, ok := strings.Cut(line, "=")

		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), "\"'")

		if key != "" {
			if _, exists := os.LookupEnv(key); !exists {
				_ = os.Setenv(key, value)
			}
		}
	}

	return scanner.Err()
}
