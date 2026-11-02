package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OpenAIEmbedder struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

type OllamaEmbedder struct {
	model   string
	baseURL string
	client  *http.Client
}

type OllamaEmbedderConfig struct {
	Model   string
	BaseURL string
	Client  *http.Client
}

func NewOllamaEmbedder(config OllamaEmbedderConfig) (*OllamaEmbedder, error) {
	if strings.TrimSpace(config.Model) == "" {
		return nil, fmt.Errorf("Ollama embedding model is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:11434"
	}
	client := config.Client
	if client == nil {
		client = &http.Client{}
	}
	return &OllamaEmbedder{model: config.Model, baseURL: baseURL, client: client}, nil
}

func (e *OllamaEmbedder) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	body, err := json.Marshal(map[string]any{"model": e.model, "input": inputs})
	if err != nil {
		return nil, fmt.Errorf("marshal Ollama embedding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/api/embed", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create Ollama embedding request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send Ollama embedding request: %w", err)
	}
	defer res.Body.Close()
	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read Ollama embedding response: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("Ollama embedding API returned %s: %s", res.Status, strings.TrimSpace(string(responseBody)))
	}
	var response struct {
		Embeddings [][]float32 `json:"embeddings"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("decode Ollama embedding response: %w", err)
	}
	return response.Embeddings, nil
}

type OpenAIEmbedderConfig struct {
	APIKey  string
	Model   string
	BaseURL string
	Client  *http.Client
}

func NewOpenAIEmbedder(config OpenAIEmbedderConfig) (*OpenAIEmbedder, error) {
	if strings.TrimSpace(config.APIKey) == "" {
		return nil, fmt.Errorf("OpenAI API key is required for embeddings")
	}
	if strings.TrimSpace(config.Model) == "" {
		return nil, fmt.Errorf("embedding model is required")
	}
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultOpenAIBaseURL
	}
	client := config.Client
	if client == nil {
		client = &http.Client{}
	}
	return &OpenAIEmbedder{apiKey: config.APIKey, model: config.Model, baseURL: baseURL, client: client}, nil
}

func (e *OpenAIEmbedder) Embed(ctx context.Context, inputs []string) ([][]float32, error) {
	body, err := json.Marshal(map[string]any{"model": e.model, "input": inputs})
	if err != nil {
		return nil, fmt.Errorf("marshal embedding request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL+"/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create embedding request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+e.apiKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send embedding request: %w", err)
	}
	defer res.Body.Close()
	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("read embedding response: %w", err)
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("embedding API returned %s: %s", res.Status, strings.TrimSpace(string(responseBody)))
	}
	var response struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(responseBody, &response); err != nil {
		return nil, fmt.Errorf("decode embedding response: %w", err)
	}
	vectors := make([][]float32, len(response.Data))
	for i := range response.Data {
		vectors[i] = response.Data[i].Embedding
	}
	return vectors, nil
}
