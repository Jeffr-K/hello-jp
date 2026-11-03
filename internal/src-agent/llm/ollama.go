package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OllamaProvider struct {
	model   string
	baseURL string
	client  *http.Client
}

type OllamaProviderConfig struct {
	Model   string
	BaseURL string
	Client  *http.Client
}

func NewOllamaProvider(config OllamaProviderConfig) (*OllamaProvider, error) {
	if strings.TrimSpace(config.Model) == "" {
		return nil, fmt.Errorf("Ollama model is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	client := config.Client
	if client == nil {
		client = &http.Client{}
	}
	return &OllamaProvider{model: config.Model, baseURL: baseURL, client: client}, nil
}

func (p *OllamaProvider) Complete(ctx context.Context, request Request) (Response, error) {
	payload := ollamaChatRequest{Model: p.model, Stream: false, Messages: make([]ollamaMessage, 0, len(request.Messages))}
	for _, message := range request.Messages {
		converted := ollamaMessage{Role: string(message.Role), Content: message.Content}
		for _, call := range message.ToolCalls {
			var arguments map[string]any
			if err := json.Unmarshal(call.Arguments, &arguments); err != nil {
				return Response{}, fmt.Errorf("decode Ollama tool arguments: %w", err)
			}
			converted.ToolCalls = append(converted.ToolCalls, ollamaToolCall{Function: ollamaFunction{Name: call.Name, Arguments: arguments}})
		}
		payload.Messages = append(payload.Messages, converted)
	}
	for _, tool := range request.Tools {
		payload.Tools = append(payload.Tools, ollamaTool{Type: "function", Function: ollamaToolFunction{Name: tool.Name, Description: tool.Description, Parameters: tool.InputSchema}})
	}
	if len(payload.Tools) == 0 {
		payload.Tools = nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshal Ollama request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create Ollama request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := p.client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("send Ollama request: %w", err)
	}
	defer res.Body.Close()
	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read Ollama response: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Response{}, fmt.Errorf("Ollama returned %s: %s", res.Status, strings.TrimSpace(string(responseBody)))
	}
	var response ollamaChatResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return Response{}, fmt.Errorf("decode Ollama response: %w", err)
	}
	message := Message{Role: Role(response.Message.Role), Content: response.Message.Content}
	for index, call := range response.Message.ToolCalls {
		arguments, err := json.Marshal(call.Function.Arguments)
		if err != nil {
			return Response{}, fmt.Errorf("encode Ollama tool arguments: %w", err)
		}
		message.ToolCalls = append(message.ToolCalls, ToolCall{ID: fmt.Sprintf("ollama-call-%d", index+1), Name: call.Function.Name, Arguments: arguments})
	}
	return Response{Message: message}, nil
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Tools    []ollamaTool    `json:"tools,omitempty"`
	Stream   bool            `json:"stream"`
}

func (p *OllamaProvider) Stream(ctx context.Context, request Request, onDelta func(string) error) (Response, error) {
	payload := ollamaChatRequest{Model: p.model, Stream: true, Messages: make([]ollamaMessage, 0, len(request.Messages))}
	for _, message := range request.Messages {
		converted := ollamaMessage{Role: string(message.Role), Content: message.Content}
		payload.Messages = append(payload.Messages, converted)
	}
	for _, tool := range request.Tools {
		payload.Tools = append(payload.Tools, ollamaTool{Type: "function", Function: ollamaToolFunction{Name: tool.Name, Description: tool.Description, Parameters: tool.InputSchema}})
	}
	if len(payload.Tools) == 0 {
		payload.Tools = nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshal Ollama streaming request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create Ollama streaming request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := p.client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("send Ollama streaming request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(res.Body)
		return Response{}, fmt.Errorf("Ollama streaming request returned %s: %s", res.Status, strings.TrimSpace(string(responseBody)))
	}

	var content strings.Builder
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		var chunk ollamaChatResponse
		if err := json.Unmarshal(scanner.Bytes(), &chunk); err != nil {
			return Response{}, fmt.Errorf("decode Ollama stream: %w", err)
		}
		delta := chunk.Message.Content
		if delta != "" {
			content.WriteString(delta)
			if err := onDelta(delta); err != nil {
				return Response{}, err
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Response{}, fmt.Errorf("read Ollama stream: %w", err)
	}
	return Response{Message: Message{Role: RoleAssistant, Content: content.String()}}, nil
}

type ollamaMessage struct {
	Role      string           `json:"role"`
	Content   string           `json:"content,omitempty"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}
type ollamaToolCall struct {
	Function ollamaFunction `json:"function"`
}
type ollamaFunction struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}
type ollamaTool struct {
	Type     string             `json:"type"`
	Function ollamaToolFunction `json:"function"`
}
type ollamaToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}
type ollamaChatResponse struct {
	Message ollamaMessage `json:"message"`
}
