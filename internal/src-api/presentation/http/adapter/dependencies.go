package adapter

import (
	"github.com/meshestra/go-playground/jp/internal/auth"
	"github.com/meshestra/go-playground/jp/internal/src-api/application"
)

// Dependencies is the composition boundary between adapters and the API.
type Dependencies struct {
	Chat      application.ChatRunner
	Auth      *auth.Service
	Generator application.UserContentGenerator
}
