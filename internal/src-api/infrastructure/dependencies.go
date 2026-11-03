package infrastructure

import (
	"github.com/meshestra/go-playground/jp/internal/auth"
	"github.com/meshestra/go-playground/jp/internal/src-api/application"
)

// Dependencies is the composition boundary between adapters and the API.
type Dependencies struct {
	Auth      *auth.Service
	Generator application.UserContentGenerator
}
