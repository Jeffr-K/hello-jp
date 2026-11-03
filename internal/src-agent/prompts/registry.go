package prompts

import (
	"context"
	"fmt"
	"sort"
	"sync"
)

type Registry struct {
	mu          sync.RWMutex
	definitions map[string]PromptDefinition
	renderer    *Renderer
}

func NewRegistry(definitions []PromptDefinition, renderer *Renderer) (*Registry, error) {
	if renderer == nil {
		return nil, fmt.Errorf("prompt renderer is required")
	}
	r := &Registry{definitions: make(map[string]PromptDefinition), renderer: renderer}
	for _, definition := range definitions {
		if err := r.Register(definition); err != nil {
			return nil, err
		}
	}
	return r, nil
}

func (r *Registry) Register(definition PromptDefinition) error {
	if definition.ID == "" || definition.Version == "" || definition.Template == "" {
		return fmt.Errorf("prompt ID, version, and template are required")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.definitions[definition.ID]; exists {
		return fmt.Errorf("prompt %q is already registered", definition.ID)
	}
	r.definitions[definition.ID] = definition
	return nil
}

func (r *Registry) Get(id string) (PromptDefinition, error) {
	r.mu.RLock()
	definition, ok := r.definitions[id]
	r.mu.RUnlock()
	if !ok {
		return PromptDefinition{}, fmt.Errorf("prompt %q is not registered", id)
	}
	return definition, nil
}

func (r *Registry) IDs() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ids := make([]string, 0, len(r.definitions))
	for id := range r.definitions {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func (r *Registry) Render(ctx context.Context, id string, data any) (RenderedPrompt, error) {
	definition, err := r.Get(id)
	if err != nil {
		return RenderedPrompt{}, err
	}
	content, err := r.renderer.Render(ctx, definition.Template, data)
	if err != nil {
		return RenderedPrompt{}, err
	}
	return RenderedPrompt{Definition: definition, Content: content}, nil
}
