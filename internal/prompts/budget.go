package prompts

import (
	"fmt"
	"unicode/utf8"

	"github.com/meshestra/go-playground/jp/internal/llm"
)

type Budget struct {
	ContextWindow int
	OutputReserve int
	SafetyMargin  int
}

type Usage struct {
	EstimatedTokens int
	AvailableTokens int
}

func (b Budget) Check(messages []llm.Message) (Usage, error) {
	if b.ContextWindow <= 0 {
		return Usage{}, fmt.Errorf("context window must be positive")
	}
	if b.OutputReserve < 0 || b.SafetyMargin < 0 {
		return Usage{}, fmt.Errorf("budget reserves cannot be negative")
	}
	total := 0
	for _, message := range messages {
		total += estimateTokens(message.Content) + 4
	}
	available := b.ContextWindow - b.OutputReserve - b.SafetyMargin
	if total > available {
		return Usage{EstimatedTokens: total, AvailableTokens: available}, fmt.Errorf("prompt budget exceeded: estimated %d tokens, available %d", total, available)
	}
	return Usage{EstimatedTokens: total, AvailableTokens: available}, nil
}

func estimateTokens(value string) int { return (utf8.RuneCountInString(value) + 3) / 4 }
