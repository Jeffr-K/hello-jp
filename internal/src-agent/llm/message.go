package llm

import (
	"context"
	"encoding/json"
)

type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

type Message struct {
	Role       Role       `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`
}

type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolDefinition struct {
	Name        string
	Description string
	InputSchema json.RawMessage
}

type Request struct {
	Messages []Message
	Tools    []ToolDefinition
}

type Response struct {
	Message      Message
	FinishReason string
}

type Provider interface {
	Complete(ctx context.Context, request Request) (Response, error)
}

type Streamer interface {
	Stream(ctx context.Context, request Request, onDelta func(string) error) (Response, error)
}
