package agent

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/meshestra/go-playground/jp/internal/llm"
	"github.com/meshestra/go-playground/jp/internal/memory"
	"github.com/meshestra/go-playground/jp/internal/tools"
)

type fakeProvider struct {
	mu        sync.Mutex
	responses []llm.Response
	requests  []llm.Request
}

func (p *fakeProvider) Complete(ctx context.Context, request llm.Request) (llm.Response, error) {
	if err := ctx.Err(); err != nil {
		return llm.Response{}, err
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	p.requests = append(p.requests, request)
	if len(p.responses) == 0 {
		return llm.Response{}, errors.New("fake provider has no response")
	}

	response := p.responses[0]
	p.responses = p.responses[1:]

	return response, nil
}

type fakeTool struct {
	name    string
	result  string
	err     error
	calls   int
	lastArg json.RawMessage
}

func (t *fakeTool) Name() string {
	return t.name
}

func (t *fakeTool) Description() string {
	return "fake test tool"
}

func (t *fakeTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{"type":"object","additionalProperties":false,"properties":{"text":{"type":"string"}}}`)
}

func (t *fakeTool) Execute(ctx context.Context, call tools.Call) (tools.Result, error) {
	if err := ctx.Err(); err != nil {
		return tools.Result{}, err
	}

	t.calls++
	t.lastArg = append([]byte(nil), call.Arguments...)
	if t.err != nil {
		return tools.Result{}, t.err
	}

	return tools.Result{Content: t.result}, nil
}

func TestRuntimeReturnsDirectLLMResponse(t *testing.T) {
	provider := &fakeProvider{responses: []llm.Response{{
		Message: llm.Message{Content: "안녕하세요"},
	}}}
	store := memory.NewInMemoryStore()
	runtime, err := NewRuntime(RuntimeConfig{Provider: provider, Memory: store})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}

	response, err := runtime.Run(context.Background(), "user-1", "session-1", "인사해줘")
	if err != nil {
		t.Fatalf("run runtime: %v", err)
	}

	if response.Content != "안녕하세요" {
		t.Fatalf("unexpected response: %q", response.Content)
	}

	messages, err := store.LoadMessages(context.Background(), "user-1", "session-1", 0)
	if err != nil {
		t.Fatalf("load messages: %v", err)
	}

	if len(messages) != 2 {
		t.Fatalf("expected user and assistant messages, got %d", len(messages))
	}
}

func TestRuntimeLoadsStructuredLearnerMemory(t *testing.T) {
	provider := &fakeProvider{responses: []llm.Response{{
		Message: llm.Message{Content: "맞춤 응답"},
	}}}
	store := memory.NewInMemoryStore()
	if err := store.SaveLearningProfile(context.Background(), "user-1", memory.LearningProfile{
		Level:       "N3",
		WeakGrammar: []string{"조사"},
	}); err != nil {
		t.Fatalf("save learning profile: %v", err)
	}
	if err := store.AddEpisode(context.Background(), "user-1", memory.Episode{
		SessionID: "old-session",
		Topic:     "운동",
		Score:     0.8,
	}); err != nil {
		t.Fatalf("save learning episode: %v", err)
	}

	runtime, err := NewRuntime(RuntimeConfig{Provider: provider, Memory: store})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}

	if _, err := runtime.Run(context.Background(), "user-1", "session-1", "오늘 공부할 내용을 추천해줘"); err != nil {
		t.Fatalf("run runtime: %v", err)
	}

	if len(provider.requests) != 1 || len(provider.requests[0].Messages) < 2 {
		t.Fatalf("expected memory context and user message: %+v", provider.requests)
	}

	if provider.requests[0].Messages[0].Role != llm.RoleSystem {
		t.Fatalf("expected system memory context, got %s", provider.requests[0].Messages[0].Role)
	}

	if !strings.Contains(provider.requests[0].Messages[0].Content, "WeakGrammar") && !strings.Contains(provider.requests[0].Messages[0].Content, "weak_grammar") {
		t.Fatalf("memory context does not include profile: %s", provider.requests[0].Messages[0].Content)
	}
}

func TestRuntimeExecutesMultipleToolCalls(t *testing.T) {
	provider := &fakeProvider{responses: []llm.Response{
		{Message: llm.Message{
			ToolCalls: []llm.ToolCall{
				{ID: "call-1", Name: "translate", Arguments: json.RawMessage(`{"text":"안녕"}`)},
				{ID: "call-2", Name: "grammar", Arguments: json.RawMessage(`{"text":"食べます"}`)},
			},
		}},
		{Message: llm.Message{Content: "도구 결과를 반영했습니다."}},
	}}
	store := memory.NewInMemoryStore()
	registry := tools.NewRegistry()
	translate := &fakeTool{name: "translate", result: "hello"}
	grammar := &fakeTool{name: "grammar", result: "polite form"}
	if err := registry.Register(translate); err != nil {
		t.Fatalf("register translate: %v", err)
	}
	if err := registry.Register(grammar); err != nil {
		t.Fatalf("register grammar: %v", err)
	}

	runtime, err := NewRuntime(RuntimeConfig{Provider: provider, Memory: store, Tools: registry})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}

	response, err := runtime.Run(context.Background(), "user-1", "session-1", "분석해줘")
	if err != nil {
		t.Fatalf("run runtime: %v", err)
	}

	if response.Content != "도구 결과를 반영했습니다." {
		t.Fatalf("unexpected response: %q", response.Content)
	}

	if translate.calls != 1 || grammar.calls != 1 {
		t.Fatalf("expected both tools to execute once, got translate=%d grammar=%d", translate.calls, grammar.calls)
	}

	messages, err := store.LoadMessages(context.Background(), "user-1", "session-1", 0)
	if err != nil {
		t.Fatalf("load messages: %v", err)
	}

	if len(messages) != 5 {
		t.Fatalf("expected user, assistant, 2 tools, assistant; got %d messages", len(messages))
	}
}

func TestRuntimeRejectsInvalidToolArguments(t *testing.T) {
	provider := &fakeProvider{responses: []llm.Response{{
		Message: llm.Message{ToolCalls: []llm.ToolCall{
			{ID: "call-1", Name: "translate", Arguments: json.RawMessage(`[]`)},
		}},
	}}}

	registry := tools.NewRegistry()
	if err := registry.Register(&fakeTool{name: "translate", result: "hello"}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	runtime, err := NewRuntime(RuntimeConfig{
		Provider: provider,
		Memory:   memory.NewInMemoryStore(),
		Tools:    registry,
	})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}

	_, err = runtime.Run(context.Background(), "user-1", "session-1", "번역해줘")
	if err == nil || !strings.Contains(err.Error(), "arguments must be a JSON object") {
		t.Fatalf("expected invalid argument error, got %v", err)
	}
}

func TestRuntimePropagatesToolError(t *testing.T) {
	provider := &fakeProvider{responses: []llm.Response{{
		Message: llm.Message{ToolCalls: []llm.ToolCall{
			{ID: "call-1", Name: "translate", Arguments: json.RawMessage(`{}`)},
		}},
	}}}

	registry := tools.NewRegistry()
	if err := registry.Register(&fakeTool{name: "translate", err: errors.New("translation service unavailable")}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	runtime, err := NewRuntime(RuntimeConfig{
		Provider: provider,
		Memory:   memory.NewInMemoryStore(),
		Tools:    registry,
	})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}

	_, err = runtime.Run(context.Background(), "user-1", "session-1", "번역해줘")
	if err == nil || !strings.Contains(err.Error(), "translation service unavailable") {
		t.Fatalf("expected tool error, got %v", err)
	}
}

func TestRuntimeStopsAtMaximumIterations(t *testing.T) {
	provider := &fakeProvider{responses: []llm.Response{
		{Message: llm.Message{ToolCalls: []llm.ToolCall{{ID: "call-1", Name: "translate", Arguments: json.RawMessage(`{}`)}}}},
		{Message: llm.Message{ToolCalls: []llm.ToolCall{{ID: "call-2", Name: "translate", Arguments: json.RawMessage(`{}`)}}}},
	}}
	registry := tools.NewRegistry()
	if err := registry.Register(&fakeTool{name: "translate", result: "hello"}); err != nil {
		t.Fatalf("register tool: %v", err)
	}

	runtime, err := NewRuntime(RuntimeConfig{
		Provider:      provider,
		Memory:        memory.NewInMemoryStore(),
		Tools:         registry,
		MaxIterations: 2,
	})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}

	_, err = runtime.Run(context.Background(), "user-1", "session-1", "계속 실행해줘")
	if err == nil || !strings.Contains(err.Error(), "maximum iterations") {
		t.Fatalf("expected max iteration error, got %v", err)
	}
}

func TestRuntimeHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	provider := &fakeProvider{responses: []llm.Response{{Message: llm.Message{Content: "응답"}}}}
	runtime, err := NewRuntime(RuntimeConfig{Provider: provider, Memory: memory.NewInMemoryStore()})
	if err != nil {
		t.Fatalf("create runtime: %v", err)
	}

	_, err = runtime.Run(ctx, "user-1", "session-1", "취소 테스트")
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Fatalf("expected cancellation error, got %v", err)
	}
}

func TestRuntimeValidatesStructuredOutputWithLimitedRetry(t *testing.T) {
	provider := &fakeProvider{responses: []llm.Response{
		{Message: llm.Message{Content: "not json"}},
		{Message: llm.Message{Content: `{"translation":"こんにちは","formality":"polite","grammar":[],"alternatives":[]}`}},
	}}
	runtime, err := NewRuntime(RuntimeConfig{
		Provider:          provider,
		Memory:            memory.NewInMemoryStore(),
		StructuredOutput:  true,
		ValidationRetries: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	response, err := runtime.Run(context.Background(), "user-1", "session-1", "번역해줘")
	if err != nil {
		t.Fatalf("run runtime: %v", err)
	}
	if response.Content == "not json" || len(provider.requests) != 2 {
		t.Fatalf("expected one validation retry, response=%q requests=%d", response.Content, len(provider.requests))
	}
}
