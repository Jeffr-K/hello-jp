package agent

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/meshestra/go-playground/jp/internal/config"
	"github.com/meshestra/go-playground/jp/internal/content"
	"github.com/meshestra/go-playground/jp/internal/retrieval"
)

type Agent struct {
	config config.Config
	client *http.Client
}

func New() (*Agent, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	return &Agent{
		config: cfg,
		// Local models may spend several minutes loading on their first request.
		// The request context still allows callers to cancel earlier.
		client: &http.Client{Timeout: 10 * time.Minute},
	}, nil
}

func (a *Agent) Run(ctx context.Context, topic string) (*content.DailyContent, error) {
	if a.config.LLMProvider == "ollama" {
		return a.runOllama(ctx, topic)
	}
	if strings.TrimSpace(a.config.OpenAIAPIKey) == "" {
		return nil, fmt.Errorf("OPENAI_API_KEY is not set")
	}

	today := time.Now().Format("2006-01-02")
	requestTopic := buildDailyTopic(topic)
	payload := map[string]any{
		"model": a.config.LLMModel,
		"input": []map[string]string{
			{"role": "system", "content": systemPrompt(a.config.DailyLevel)},
			{"role": "user", "content": fmt.Sprintf("날짜: %s\n학습 주제: %s", today, requestTopic)},
		},
		"text": map[string]any{
			"format": map[string]any{
				"type":   "json_schema",
				"name":   "daily_japanese_content",
				"strict": true,
				"schema": dailySchema(),
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		"https://api.openai.com/v1/responses",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+a.config.OpenAIAPIKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call OpenAI Responses API: %w", err)
	}

	defer res.Body.Close()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("OpenAI API returned %s: %s", res.Status, strings.TrimSpace(string(responseBody)))
	}

	var response responseEnvelope

	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("decode OpenAI response: %w", err)
	}

	text := response.OutputText()
	if text == "" {
		return nil, fmt.Errorf("OpenAI response did not contain output text")
	}

	var daily content.DailyContent

	if err := json.Unmarshal([]byte(text), &daily); err != nil {
		return nil, fmt.Errorf("decode generated content: %w", err)
	}

	daily.Date = today

	if err := daily.Validate(); err != nil {
		return nil, fmt.Errorf("validate generated content: %w", err)
	}

	if err := save(a.config.ContentDir, &daily); err != nil {
		return nil, err
	}

	return &daily, nil
}

// RunForUser generates a lesson and stores a user-scoped copy so authenticated
// learners can keep different daily topics without overwriting each other.
func (a *Agent) RunForUser(ctx context.Context, userID, topic string) (*content.DailyContent, error) {
	daily, err := a.Run(ctx, topic)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(filepath.Join(a.config.ContentDir, daily.Date+".json"))
	if err != nil {
		return nil, fmt.Errorf("read generated content: %w", err)
	}
	directory := filepath.Join(a.config.ContentDir, "users", userID)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return nil, fmt.Errorf("create user content directory: %w", err)
	}
	if err := os.WriteFile(filepath.Join(directory, daily.Date+".json"), data, 0o644); err != nil {
		return nil, fmt.Errorf("save user content: %w", err)
	}
	return daily, nil
}

func (a *Agent) IndexDaily(ctx context.Context, daily *content.DailyContent) error {
	if a.config.QdrantURL == "" || daily == nil {
		return nil
	}
	embedder, err := newEmbedder(a.config)
	if err != nil {
		return err
	}
	index, err := retrieval.NewQdrantIndex(a.config.QdrantURL, a.config.QdrantCollection, embedder, nil)
	if err != nil {
		return err
	}
	return index.Upsert(ctx, retrieval.ChunksFromDocuments(retrieval.DocumentsFromDailyContent(*daily)))
}

func (a *Agent) runOllama(ctx context.Context, topic string) (*content.DailyContent, error) {
	today := time.Now().Format("2006-01-02")
	requestTopic := buildDailyTopic(topic)
	payload := map[string]any{
		"model":  a.config.LLMModel,
		"stream": false,
		"think":  false,
		"format": dailySchema(),
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt(a.config.DailyLevel)},
			{"role": "user", "content": fmt.Sprintf("날짜: %s\n학습 주제: %s", today, requestTopic)},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal Ollama request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(a.config.OllamaBaseURL, "/")+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Ollama request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	res, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call Ollama API: %w", err)
	}

	defer res.Body.Close()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read Ollama response: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("Ollama API returned %s: %s", res.Status, strings.TrimSpace(string(responseBody)))
	}

	var response struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}

	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("decode Ollama response: %w", err)
	}
	if strings.TrimSpace(response.Message.Content) == "" {
		return nil, fmt.Errorf("Ollama response did not contain message content")
	}
	var daily content.DailyContent
	if err := json.Unmarshal([]byte(response.Message.Content), &daily); err != nil {
		return nil, fmt.Errorf("decode Ollama generated content: %w", err)
	}

	daily.Date = today
	if err := daily.Validate(); err != nil {
		return nil, fmt.Errorf("validate generated content: %w", err)
	}
	if err := save(a.config.ContentDir, &daily); err != nil {
		return nil, err
	}

	return &daily, nil
}

func buildDailyTopic(topic string) string {
	focuses := []string{"스트레칭과 준비 운동", "달리기와 걷기", "근력 운동과 자세", "운동 강도와 체력", "운동 습관과 일정", "운동 후 회복과 휴식", "수분 섭취와 식사", "날씨와 야외 운동", "운동 목표와 동기 부여", "친구와 함께하는 운동"}
	index := 0
	if value, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(len(focuses)))); err == nil {
		index = int(value.Int64())
	}
	return fmt.Sprintf("%s\n세부 주제는 '%s'로 선택한다. 이전 생성물과 표현과 문법이 겹치지 않도록 다양한 단어와 문장 유형을 사용한다.", topic, focuses[index])
}

type responseEnvelope struct {
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

func (r responseEnvelope) OutputText() string {
	for _, item := range r.Output {
		for _, part := range item.Content {
			if part.Type == "output_text" {
				return strings.TrimSpace(part.Text)
			}
		}
	}

	return ""
}

func save(dir string, daily *content.DailyContent) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("create content directory: %w", err)
	}

	data, err := json.MarshalIndent(daily, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal generated content: %w", err)
	}

	path := filepath.Join(dir, daily.Date+".json")

	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("save generated content: %w", err)
	}

	return nil
}

func systemPrompt(level string) string {
	return fmt.Sprintf(`너는 한국어 학습자를 위한 일본어 콘텐츠 에디터다.
JLPT %s 수준에 맞춰 오늘의 일본어 학습 세트를 만든다.
큰 주제가 같아도 전달된 세부 주제를 중심으로 새로운 어휘, 문법, 상황을 선택한다. 이전 결과를 그대로 반복하지 않는다.
반드시 지정된 JSON 스키마만 출력한다. JSON 외의 설명은 출력하지 않는다.
단어 3~5개, 문장 1~3개, 문법 1개, 실전 대화 4~6턴, 연습 문제 2~3개를 만든다.
오늘의 학습은 하나의 실제 상황을 중심으로 구성한다. situation에는 장소와 상황을 짧게 쓴다.
dialogue는 해당 상황에서 바로 사용할 수 있는 자연스러운 일본어 대화로 만든다. speaker는 'A' 또는 'B'를 사용한다.
culture에는 주제와 연결되는 일본의 역사·문화 맥락을 짧고 검증 가능한 수준으로 작성한다. 확실하지 않은 연도나 사실은 만들지 않는다.
visual.prompt에는 오늘의 상황을 표현할 이미지 생성용 설명을, visual.alt에는 접근성 설명을 작성한다. visual.url은 기본적으로 빈 문자열로 둔다.
일본어는 자연스럽고 실제 일상에서 사용하는 표현으로 작성한다.
각 일본어 항목에는 정확한 히라가나 읽기와 자연스러운 한국어 번역을 제공한다.
moments_text는 HelloTalk Moments에 바로 붙여넣을 수 있는 짧고 읽기 쉬운 한국어/일본어 게시물로 만든다.
문법 설명은 초보 학습자도 이해할 수 있는 한국어로 자세히 작성한다.
문법은 반드시 다음 순서로 설명한다: 전체 뜻, 문장 구조를 괄호로 표시한 공식, 각 단어의 역할과 기본형, 예문 속 활용, 추가 예문 2개.
grammar.explanation 안에서는 각 항목을 반드시 줄바꿈으로 나눈다. '전체 뜻:', '문장 구조:', '단어별 설명:', '예문 속 활용:', '추가 예문:'을 각각 새 줄에서 시작하고, 단어별 설명과 예문은 한 줄에 하나씩 작성한다.
grammar.pattern에는 한국어 단어를 넣지 말고 일본어 구조만 작성한다. 예: 'AはBより少し多いです'. 조사와 표현의 뜻은 explanation에서 설명한다.
です는 상황에 따라 정중한 문장 끝 표현이라고 설명하며 '말씀'처럼 부정확한 뜻으로 번역하지 않는다.
예를 들어 '昨日より少し多い'는 '昨日(어제) + より(~보다) + 少し(조금) + 多い(많다)'로 나누고, 'AよりBが多い' 또는 'AはBより多い'처럼 실제 어순을 보여준다.
より는 '~보다'라는 비교 기준을 만드는 조사이지 어려운 '비교급' 용어로 설명하지 않는다. 多い는 い형용사 '많다'의 기본형이라고 설명한다.
일본어 표현을 잘못 읽을 수 있는 비슷한 소리가 있으면 반드시 히라가나와 한국어 뜻을 함께 제시한다.
근거 없는 JLPT 공식 문법 분류, 영어식 문법 용어, '(JL: ...)'처럼 정의되지 않은 표기를 만들지 않는다.
`, level)
}

func dailySchema() map[string]any {
	stringField := func() map[string]any {
		return map[string]any{"type": "string"}
	}

	sentence := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"japanese":    stringField(),
			"reading":     stringField(),
			"translation": stringField(),
		},
		"required": []string{"japanese", "reading", "translation"},
	}

	word := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"word":                stringField(),
			"reading":             stringField(),
			"meaning":             stringField(),
			"example":             stringField(),
			"example_translation": stringField(),
		},
		"required": []string{"word", "reading", "meaning", "example", "example_translation"},
	}

	dialogueTurn := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"speaker": stringField(), "japanese": stringField(),
			"reading": stringField(), "translation": stringField(),
		},
		"required": []string{"speaker", "japanese", "reading", "translation"},
	}
	practice := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties":           map[string]any{"type": stringField(), "prompt": stringField(), "answer": stringField()},
		"required":             []string{"type", "prompt", "answer"},
	}

	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"date":      stringField(),
			"level":     stringField(),
			"theme":     stringField(),
			"situation": stringField(),
			"words":     map[string]any{"type": "array", "items": word},
			"sentences": map[string]any{"type": "array", "items": sentence},
			"dialogue":  map[string]any{"type": "array", "items": dialogueTurn},
			"culture": map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": map[string]any{"title": stringField(), "summary": stringField(), "period": stringField(), "source": stringField()},
				"required":   []string{"title", "summary", "period", "source"},
			},
			"visual": map[string]any{
				"type": "object", "additionalProperties": false,
				"properties": map[string]any{"prompt": stringField(), "alt": stringField(), "url": stringField()},
				"required":   []string{"prompt", "alt", "url"},
			},
			"practice": map[string]any{"type": "array", "items": practice},
			"grammar": map[string]any{
				"type":                 "object",
				"additionalProperties": false,
				"properties": map[string]any{
					"pattern":     stringField(),
					"meaning":     stringField(),
					"explanation": stringField(),
					"examples":    map[string]any{"type": "array", "items": sentence},
				},
				"required": []string{"pattern", "meaning", "explanation", "examples"},
			},
			"moments_text": stringField(),
		},
		"required": []string{"date", "level", "theme", "situation", "words", "sentences", "dialogue", "culture", "visual", "grammar", "practice", "moments_text"},
	}
}
