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

const defaultOpenAIBaseURL = "https://api.openai.com/v1"

type OpenAIProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

type OpenAIProviderConfig struct {
	APIKey  string
	Model   string
	BaseURL string
	Client  *http.Client
}

func NewOpenAIProvider(config OpenAIProviderConfig) (*OpenAIProvider, error) {
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	if strings.TrimSpace(config.Model) == "" {
		return nil, fmt.Errorf("OpenAI model is required")
	}

	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}

	client := config.Client
	if client == nil {
		client = &http.Client{}
	}

	return &OpenAIProvider{
		apiKey:  config.APIKey,
		model:   config.Model,
		baseURL: baseURL,
		client:  client,
	}, nil
}

func (p *OpenAIProvider) Complete(ctx context.Context, request Request) (Response, error) {
	payload := chatCompletionRequest{
		Model:    p.model,
		Messages: make([]chatMessage, 0, len(request.Messages)),
		Tools:    make([]chatTool, 0, len(request.Tools)),
	}

	for _, message := range request.Messages {
		payload.Messages = append(payload.Messages, toChatMessage(message))
	}

	for _, tool := range request.Tools {
		payload.Tools = append(payload.Tools, chatTool{
			Type: "function",
			Function: chatFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}

	if len(payload.Tools) == 0 {
		payload.Tools = nil
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshal chat completion request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return Response{}, fmt.Errorf("create chat completion request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := p.client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("send chat completion request: %w", err)
	}
	defer res.Body.Close()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return Response{}, fmt.Errorf("read chat completion response: %w", err)
	}

	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return Response{}, fmt.Errorf("OpenAI chat completion returned %s: %s", res.Status, strings.TrimSpace(string(responseBody)))
	}

	var response chatCompletionResponse
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return Response{}, fmt.Errorf("decode chat completion response: %w", err)
	}

	if len(response.Choices) == 0 {
		return Response{}, fmt.Errorf("chat completion response has no choices")
	}

	message := fromChatMessage(response.Choices[0].Message)

	return Response{
		Message:      message,
		FinishReason: response.Choices[0].FinishReason,
	}, nil
}

type chatCompletionRequest struct {
	Model    string        `json:"model"`
	Messages []chatMessage `json:"messages"`
	Tools    []chatTool    `json:"tools,omitempty"`
	Stream   bool          `json:"stream,omitempty"`
}

type chatMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content,omitempty"`
	ToolCalls  []chatToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
	Name       string         `json:"name,omitempty"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatFunction `json:"function"`
}

type chatFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type chatToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function chatCallFunction `json:"function"`
}

type chatCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
}

func (p *OpenAIProvider) Stream(ctx context.Context, request Request, onDelta func(string) error) (Response, error) {
	payload := chatCompletionRequest{
		Model:    p.model,
		Messages: make([]chatMessage, 0, len(request.Messages)),
		Tools:    make([]chatTool, 0, len(request.Tools)),
		Stream:   true,
	}
	for _, message := range request.Messages {
		payload.Messages = append(payload.Messages, toChatMessage(message))
	}
	for _, tool := range request.Tools {
		payload.Tools = append(payload.Tools, chatTool{Type: "function", Function: chatFunction{Name: tool.Name, Description: tool.Description, Parameters: tool.InputSchema}})
	}
	if len(payload.Tools) == 0 {
		payload.Tools = nil
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return Response{}, fmt.Errorf("marshal streaming request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return Response{}, fmt.Errorf("create streaming request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := p.client.Do(req)
	if err != nil {
		return Response{}, fmt.Errorf("send streaming request: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		responseBody, _ := io.ReadAll(res.Body)
		return Response{}, fmt.Errorf("OpenAI streaming request returned %s: %s", res.Status, strings.TrimSpace(string(responseBody)))
	}

	var content strings.Builder
	finishReason := ""
	scanner := bufio.NewScanner(res.Body)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
				FinishReason string `json:"finish_reason"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			return Response{}, fmt.Errorf("decode OpenAI stream: %w", err)
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		delta := chunk.Choices[0].Delta.Content
		if delta != "" {
			content.WriteString(delta)
			if err := onDelta(delta); err != nil {
				return Response{}, err
			}
		}
		finishReason = chunk.Choices[0].FinishReason
	}
	if err := scanner.Err(); err != nil {
		return Response{}, fmt.Errorf("read OpenAI stream: %w", err)
	}
	return Response{Message: Message{Role: RoleAssistant, Content: content.String()}, FinishReason: finishReason}, nil
}

func toChatMessage(message Message) chatMessage {
	result := chatMessage{
		Role:       string(message.Role),
		Content:    message.Content,
		ToolCallID: message.ToolCallID,
		Name:       message.Name,
	}

	for _, toolCall := range message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, chatToolCall{
			ID:   toolCall.ID,
			Type: "function",
			Function: chatCallFunction{
				Name:      toolCall.Name,
				Arguments: string(toolCall.Arguments),
			},
		})
	}

	return result
}

func fromChatMessage(message chatMessage) Message {
	result := Message{
		Role:       Role(message.Role),
		Content:    message.Content,
		ToolCallID: message.ToolCallID,
		Name:       message.Name,
	}

	for _, toolCall := range message.ToolCalls {
		result.ToolCalls = append(result.ToolCalls, ToolCall{
			ID:        toolCall.ID,
			Name:      toolCall.Function.Name,
			Arguments: json.RawMessage(toolCall.Function.Arguments),
		})
	}

	return result
}
