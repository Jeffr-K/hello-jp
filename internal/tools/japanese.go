package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/meshestra/go-playground/jp/internal/llm"
	"github.com/meshestra/go-playground/jp/internal/memory"
)

type TranslateTool struct {
	provider llm.Provider
}

func NewTranslateTool(provider llm.Provider) (*TranslateTool, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider is required")
	}

	return &TranslateTool{provider: provider}, nil
}

func (t *TranslateTool) Name() string { return "translate" }

func (t *TranslateTool) Description() string {
	return "Translate Korean and Japanese, including naturalness and speech style."
}

func (t *TranslateTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
        "type":"object",
        "additionalProperties":false,
        "properties":{
            "text":{"type":"string"},
            "source_language":{"type":"string"},
            "target_language":{"type":"string"},
            "style":{"type":"string"}
        },
        "required":["text","target_language"]
    }`)
}

func (t *TranslateTool) Execute(ctx context.Context, call Call) (Result, error) {
	var input struct {
		Text           string `json:"text"`
		SourceLanguage string `json:"source_language"`
		TargetLanguage string `json:"target_language"`
		Style          string `json:"style"`
	}
	if err := decodeInput(call, &input); err != nil {
		return Result{}, err
	}

	if strings.TrimSpace(input.Text) == "" || strings.TrimSpace(input.TargetLanguage) == "" {
		return Result{}, fmt.Errorf("text and target_language are required")
	}

	prompt := fmt.Sprintf("Translate this text. Source language: %s. Target language: %s. Style: %s. Return a concise translation with a naturalness note if useful.\nText: %s", input.SourceLanguage, input.TargetLanguage, input.Style, input.Text)
	return completeText(ctx, t.provider, "You are a precise Korean-Japanese translation assistant.", prompt)
}

type GrammarTool struct {
	provider llm.Provider
}

func NewGrammarTool(provider llm.Provider) (*GrammarTool, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider is required")
	}

	return &GrammarTool{provider: provider}, nil
}

func (t *GrammarTool) Name() string { return "grammar" }

func (t *GrammarTool) Description() string {
	return "Analyze Japanese grammar, particles, conjugation, and provide examples."
}

func (t *GrammarTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
        "type":"object",
        "additionalProperties":false,
        "properties":{"text":{"type":"string"}},
        "required":["text"]
    }`)
}

func (t *GrammarTool) Execute(ctx context.Context, call Call) (Result, error) {
	var input struct {
		Text string `json:"text"`
	}
	if err := decodeInput(call, &input); err != nil {
		return Result{}, err
	}

	if strings.TrimSpace(input.Text) == "" {
		return Result{}, fmt.Errorf("text is required")
	}

	prompt := fmt.Sprintf("Analyze this Japanese sentence. Explain grammar patterns, particles, conjugations, Korean meaning, and give one simple example.\nSentence: %s", input.Text)
	return completeText(ctx, t.provider, "You are a patient Japanese grammar teacher for Korean learners.", prompt)
}

type VocabularyTool struct {
	provider llm.Provider
}

func NewVocabularyTool(provider llm.Provider) (*VocabularyTool, error) {
	if provider == nil {
		return nil, fmt.Errorf("provider is required")
	}

	return &VocabularyTool{provider: provider}, nil
}

func (t *VocabularyTool) Name() string { return "vocabulary" }

func (t *VocabularyTool) Description() string {
	return "Explain a Japanese word, reading, meaning, JLPT level, and example sentences."
}

func (t *VocabularyTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
        "type":"object",
        "additionalProperties":false,
        "properties":{"word":{"type":"string"},"level":{"type":"string"}},
        "required":["word"]
    }`)
}

func (t *VocabularyTool) Execute(ctx context.Context, call Call) (Result, error) {
	var input struct {
		Word  string `json:"word"`
		Level string `json:"level"`
	}
	if err := decodeInput(call, &input); err != nil {
		return Result{}, err
	}

	if strings.TrimSpace(input.Word) == "" {
		return Result{}, fmt.Errorf("word is required")
	}

	prompt := fmt.Sprintf("Explain the Japanese word %q for a Korean learner. Include reading, meaning, JLPT level, register, and two example sentences. Target level: %s", input.Word, input.Level)
	return completeText(ctx, t.provider, "You are a Japanese vocabulary teacher.", prompt)
}

type ReviewTool struct {
	memory memory.Store
}

func NewReviewTool(store memory.Store) (*ReviewTool, error) {
	if store == nil {
		return nil, fmt.Errorf("memory store is required")
	}

	return &ReviewTool{memory: store}, nil
}

func (t *ReviewTool) Name() string { return "review" }

func (t *ReviewTool) Description() string {
	return "Review learner history and select personalized Japanese practice topics."
}

func (t *ReviewTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
        "type":"object",
        "additionalProperties":false,
        "properties":{"focus":{"type":"string"}}
    }`)
}

func (t *ReviewTool) Execute(ctx context.Context, call Call) (Result, error) {
	var input struct {
		Focus string `json:"focus"`
	}
	if err := decodeInput(call, &input); err != nil {
		return Result{}, err
	}

	if call.UserID == "" {
		return Result{}, fmt.Errorf("user ID is required for review")
	}

	profile, err := t.memory.GetLearningProfile(ctx, call.UserID)
	if err != nil {
		return Result{}, fmt.Errorf("load review profile: %w", err)
	}
	episodes, err := t.memory.ListEpisodes(ctx, call.UserID, 5)
	if err != nil {
		return Result{}, fmt.Errorf("load review episodes: %w", err)
	}

	data, err := json.Marshal(struct {
		Focus   string                 `json:"focus"`
		Profile memory.LearningProfile `json:"profile"`
		History []memory.Episode       `json:"history"`
	}{input.Focus, profile, episodes})
	if err != nil {
		return Result{}, fmt.Errorf("encode review context: %w", err)
	}

	return Result{Content: string(data)}, nil
}

func decodeInput(call Call, target any) error {
	if err := json.Unmarshal(call.Arguments, target); err != nil {
		return fmt.Errorf("decode %s arguments: %w", call.Name, err)
	}

	return nil
}

func completeText(ctx context.Context, provider llm.Provider, system, prompt string) (Result, error) {
	response, err := provider.Complete(ctx, llm.Request{
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: system},
			{Role: llm.RoleUser, Content: prompt},
		},
	})
	if err != nil {
		return Result{}, fmt.Errorf("complete language tool request: %w", err)
	}
	if strings.TrimSpace(response.Message.Content) == "" {
		return Result{}, fmt.Errorf("language tool response is empty")
	}

	return Result{Content: response.Message.Content}, nil
}
