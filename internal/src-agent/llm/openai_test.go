package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(request *http.Request) (*http.Response, error) {
	return f(request)
}

func TestOpenAIProviderParsesToolCalls(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		if request.URL.Path != "/v1/chat/completions" {
			t.Fatalf("unexpected path: %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected authorization header")
		}

		var payload chatCompletionRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if payload.Model != "test-model" {
			t.Fatalf("unexpected model: %s", payload.Model)
		}
		if len(payload.Tools) != 1 || payload.Tools[0].Function.Name != "translate" {
			t.Fatalf("unexpected tools: %+v", payload.Tools)
		}

		return &http.Response{
			StatusCode: http.StatusOK,
			Status:     "200 OK",
			Body: io.NopCloser(strings.NewReader(`{
            "choices": [{
                "finish_reason": "tool_calls",
                "message": {
                    "role": "assistant",
                    "content": "",
                    "tool_calls": [{
                        "id": "call-1",
                        "type": "function",
                        "function": {
                            "name": "translate",
                            "arguments": "{\"text\":\"안녕\"}"
                        }
                    }]
                }
            }]
        }`)),
			Header: make(http.Header),
		}, nil
	})}

	provider, err := NewOpenAIProvider(OpenAIProviderConfig{
		APIKey:  "test-key",
		Model:   "test-model",
		BaseURL: "https://test.invalid/v1",
		Client:  client,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	response, err := provider.Complete(context.Background(), Request{
		Messages: []Message{{Role: RoleUser, Content: "번역해줘"}},
		Tools: []ToolDefinition{{
			Name:        "translate",
			Description: "translate text",
			InputSchema: json.RawMessage(`{"type":"object"}`),
		}},
	})
	if err != nil {
		t.Fatalf("complete: %v", err)
	}

	if response.FinishReason != "tool_calls" {
		t.Fatalf("unexpected finish reason: %s", response.FinishReason)
	}
	if len(response.Message.ToolCalls) != 1 {
		t.Fatalf("unexpected tool calls: %+v", response.Message.ToolCalls)
	}
	if response.Message.ToolCalls[0].Name != "translate" {
		t.Fatalf("unexpected tool name: %s", response.Message.ToolCalls[0].Name)
	}
}

func TestOpenAIProviderReturnsAPIError(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(request *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusBadRequest,
			Status:     "400 Bad Request",
			Body:       io.NopCloser(strings.NewReader(`{"error":{"message":"bad request"}}`)),
			Header:     make(http.Header),
		}, nil
	})}

	provider, err := NewOpenAIProvider(OpenAIProviderConfig{
		APIKey:  "test-key",
		Model:   "test-model",
		BaseURL: "https://test.invalid/v1",
		Client:  client,
	})
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}

	_, err = provider.Complete(context.Background(), Request{})
	if err == nil || !strings.Contains(err.Error(), "400 Bad Request") {
		t.Fatalf("expected API error, got %v", err)
	}
}
