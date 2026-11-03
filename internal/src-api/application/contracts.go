package application

import (
	"context"

	"github.com/meshestra/go-playground/jp/internal/src-agent/content"
	"github.com/meshestra/go-playground/jp/internal/src-agent/llm"
	"github.com/meshestra/go-playground/jp/internal/src-agent/memory"
)

// ChatRunner is the application boundary used by HTTP presentation handlers.
type ChatRunner interface {
	Run(context.Context, string, string, string) (llm.Message, error)
}

type ChatHistoryRunner interface {
	History(context.Context, string, string, int) ([]llm.Message, error)
}

type LearningStateRunner interface {
	LearningState(context.Context, string, string) (memory.LearningState, error)
	SaveLearningState(context.Context, string, memory.LearningState) error
}

type UserContentGenerator interface {
	RunForUser(context.Context, string, string) (*content.DailyContent, error)
}
