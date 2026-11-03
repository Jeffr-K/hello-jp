package domain

import "github.com/meshestra/go-playground/jp/internal/auth"

type RegisterRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Name     string `json:"name"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type TopicSettingRequest struct {
	Topic     string `json:"topic"`
	ApplyDate string `json:"apply_date"`
}

type AuthResponse struct {
	Token string    `json:"token"`
	User  auth.User `json:"user"`
}
