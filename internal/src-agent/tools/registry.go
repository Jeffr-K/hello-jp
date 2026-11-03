package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"github.com/meshestra/go-playground/jp/internal/src-agent/llm"
)

type Call struct {
	ID        string
	Name      string
	Arguments json.RawMessage
	UserID    string
}

type Result struct {
	Content string
}

type Tool interface {
	Name() string
	Description() string
	InputSchema() json.RawMessage
	Execute(ctx context.Context, call Call) (Result, error)
}

type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

func (r *Registry) Register(tool Tool) error {
	if tool == nil {
		return fmt.Errorf("tool is nil")
	}

	name := tool.Name()
	if name == "" {
		return fmt.Errorf("tool name is required")
	}

	if !json.Valid(tool.InputSchema()) {
		return fmt.Errorf("tool %q has invalid input schema", name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool %q is already registered", name)
	}

	r.tools[name] = tool

	return nil
}

func (r *Registry) Definitions() []llm.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()

	definitions := make([]llm.ToolDefinition, 0, len(r.tools))
	for _, tool := range r.tools {
		definitions = append(definitions, llm.ToolDefinition{
			Name:        tool.Name(),
			Description: tool.Description(),
			InputSchema: tool.InputSchema(),
		})
	}
	sort.Slice(definitions, func(i, j int) bool {
		return definitions[i].Name < definitions[j].Name
	})

	return definitions
}

func (r *Registry) Execute(ctx context.Context, call Call) (Result, error) {
	r.mu.RLock()
	tool, exists := r.tools[call.Name]
	r.mu.RUnlock()

	if !exists {
		return Result{}, fmt.Errorf("tool %q is not allowed", call.Name)
	}

	if err := validateArguments(call.Arguments, tool.InputSchema()); err != nil {
		return Result{}, fmt.Errorf("validate arguments for tool %q: %w", call.Name, err)
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		return Result{}, fmt.Errorf("execute tool %q: %w", call.Name, err)
	}

	return result, nil
}

func validateArguments(arguments, schema json.RawMessage) error {
	if len(arguments) == 0 {
		return fmt.Errorf("arguments are required")
	}

	var object map[string]any
	if err := json.Unmarshal(arguments, &object); err != nil {
		return fmt.Errorf("arguments must be a JSON object: %w", err)
	}

	if object == nil {
		return fmt.Errorf("arguments must be a JSON object")
	}

	var definition struct {
		Properties map[string]struct {
			Type string `json:"type"`
		} `json:"properties"`
		Required             []string `json:"required"`
		AdditionalProperties *bool    `json:"additionalProperties"`
	}
	if err := json.Unmarshal(schema, &definition); err != nil {
		return fmt.Errorf("invalid tool schema: %w", err)
	}

	for _, name := range definition.Required {
		if _, exists := object[name]; !exists {
			return fmt.Errorf("missing required argument %q", name)
		}
	}

	if definition.AdditionalProperties != nil && !*definition.AdditionalProperties {
		for name := range object {
			if _, exists := definition.Properties[name]; !exists {
				return fmt.Errorf("unknown argument %q", name)
			}
		}
	}

	for name, property := range definition.Properties {
		value, exists := object[name]
		if !exists || property.Type == "" {
			continue
		}

		if !matchesJSONType(value, property.Type) {
			return fmt.Errorf("argument %q must be %s", name, property.Type)
		}
	}

	return nil
}

func matchesJSONType(value any, expected string) bool {
	switch expected {
	case "string":
		_, ok := value.(string)
		return ok
	case "number":
		_, ok := value.(float64)
		return ok
	case "boolean":
		_, ok := value.(bool)
		return ok
	case "object":
		_, ok := value.(map[string]any)
		return ok
	case "array":
		_, ok := value.([]any)
		return ok
	case "null":
		return value == nil
	default:
		return true
	}
}
