package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrEmailExists = errors.New("email already exists")

type User struct {
	ID           string    `json:"id"`
	Email        string    `json:"email"`
	Name         string    `json:"name"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
}

type TopicSetting struct {
	UserID    string    `json:"-"`
	Topic     string    `json:"topic"`
	ApplyDate string    `json:"apply_date"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Store interface {
	CreateUser(context.Context, User) error
	GetUserByEmail(context.Context, string) (User, error)
	GetUser(context.Context, string) (User, error)
	SaveTopicSetting(context.Context, TopicSetting) error
	GetTopicSetting(context.Context, string, string) (TopicSetting, error)
	ListTopicSettings(context.Context, string) ([]TopicSetting, error)
}

type Service struct {
	store     Store
	secret    []byte
	issuer    string
	audience  string
	accessTTL time.Duration
}

type Config struct {
	Secret    string
	Issuer    string
	Audience  string
	AccessTTL time.Duration
}

func NewService(store Store, config Config) (*Service, error) {
	if store == nil {
		return nil, fmt.Errorf("auth store is required")
	}
	if strings.TrimSpace(config.Secret) == "" {
		return nil, fmt.Errorf("JWT secret is required")
	}
	if config.AccessTTL <= 0 {
		config.AccessTTL = 15 * time.Minute
	}
	if config.Issuer == "" {
		config.Issuer = "hello-jp"
	}
	if config.Audience == "" {
		config.Audience = "hello-jp-web"
	}
	return &Service{store: store, secret: []byte(config.Secret), issuer: config.Issuer, audience: config.Audience, accessTTL: config.AccessTTL}, nil
}

func (s *Service) Register(ctx context.Context, email, password, name string) (User, string, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	name = strings.TrimSpace(name)
	if !strings.Contains(email, "@") || len(email) > 320 {
		return User{}, "", fmt.Errorf("valid email is required")
	}
	if len(password) < 8 {
		return User{}, "", fmt.Errorf("password must be at least 8 characters")
	}
	if name == "" {
		return User{}, "", fmt.Errorf("name is required")
	}
	if _, err := s.store.GetUserByEmail(ctx, email); err == nil {
		return User{}, "", ErrEmailExists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return User{}, "", fmt.Errorf("hash password: %w", err)
	}
	id, err := newID()
	if err != nil {
		return User{}, "", err
	}
	user := User{ID: id, Email: email, Name: name, PasswordHash: string(hash), CreatedAt: time.Now().UTC()}
	if err := s.store.CreateUser(ctx, user); err != nil {
		return User{}, "", err
	}
	token, err := s.issue(user)
	return user, token, err
}

func (s *Service) Login(ctx context.Context, email, password string) (User, string, error) {
	user, err := s.store.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(email)))
	if err != nil || bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		return User{}, "", ErrInvalidCredentials
	}
	token, err := s.issue(user)
	return user, token, err
}

func (s *Service) User(ctx context.Context, id string) (User, error) { return s.store.GetUser(ctx, id) }

func (s *Service) SaveTopic(ctx context.Context, setting TopicSetting) error {
	if setting.UserID == "" || strings.TrimSpace(setting.Topic) == "" || setting.ApplyDate == "" {
		return fmt.Errorf("user, topic, and apply date are required")
	}
	if _, err := time.Parse("2006-01-02", setting.ApplyDate); err != nil {
		return fmt.Errorf("invalid apply date")
	}
	setting.Topic = strings.TrimSpace(setting.Topic)
	if len(setting.Topic) > 200 {
		return fmt.Errorf("topic is too long")
	}
	return s.store.SaveTopicSetting(ctx, setting)
}

func (s *Service) Topic(ctx context.Context, userID, date string) (TopicSetting, error) {
	return s.store.GetTopicSetting(ctx, userID, date)
}

func (s *Service) issue(user User) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{Subject: user.ID, Issuer: s.issuer, Audience: jwt.ClaimStrings{s.audience}, IssuedAt: jwt.NewNumericDate(now), ExpiresAt: jwt.NewNumericDate(now.Add(s.accessTTL))}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secret)
}

func (s *Service) Authenticate(raw string) (string, error) {
	raw = strings.TrimSpace(strings.TrimPrefix(raw, "Bearer "))
	if raw == "" {
		return "", ErrInvalidCredentials
	}
	parsed, err := jwt.ParseWithClaims(raw, &jwt.RegisteredClaims{}, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidCredentials
		}
		return s.secret, nil
	}, jwt.WithIssuer(s.issuer), jwt.WithAudience(s.audience))
	if err != nil || !parsed.Valid {
		return "", ErrInvalidCredentials
	}
	claims, ok := parsed.Claims.(*jwt.RegisteredClaims)
	if !ok || claims.Subject == "" {
		return "", ErrInvalidCredentials
	}
	return claims.Subject, nil
}

func newID() (string, error) {
	var bytes [16]byte
	if _, err := rand.Read(bytes[:]); err != nil {
		return "", fmt.Errorf("create user ID: %w", err)
	}
	return "user_" + base64.RawURLEncoding.EncodeToString(bytes[:]), nil
}

func constantTimeEqual(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}
