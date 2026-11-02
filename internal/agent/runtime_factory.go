package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/meshestra/go-playground/jp/internal/config"
	"github.com/meshestra/go-playground/jp/internal/content"
	"github.com/meshestra/go-playground/jp/internal/llm"
	"github.com/meshestra/go-playground/jp/internal/memory"
	"github.com/meshestra/go-playground/jp/internal/repository/postgres"
	"github.com/meshestra/go-playground/jp/internal/retrieval"
	"github.com/meshestra/go-playground/jp/internal/tools"
)

func NewConfiguredRuntime(config config.Config) (*Runtime, error) {
	var provider llm.Provider
	var err error
	switch config.LLMProvider {
	case "openai":
		provider, err = llm.NewOpenAIProvider(llm.OpenAIProviderConfig{APIKey: config.OpenAIAPIKey, Model: config.LLMModel})
	case "ollama":
		provider, err = llm.NewOllamaProvider(llm.OllamaProviderConfig{Model: config.LLMModel, BaseURL: config.OllamaBaseURL})
	default:
		return nil, fmt.Errorf("unsupported LLM provider: %s", config.LLMProvider)
	}
	if err != nil {
		return nil, fmt.Errorf("create LLM provider: %w", err)
	}

	var store memory.Store = memory.NewInMemoryStore()
	var closeStore func() error
	var retriever retrieval.Index
	if config.QdrantURL != "" {
		embedder, err := newEmbedder(config)
		if err != nil {
			return nil, err
		}
		index, err := retrieval.NewQdrantIndex(config.QdrantURL, config.QdrantCollection, embedder, nil)
		if err != nil {
			return nil, fmt.Errorf("create Qdrant index: %w", err)
		}
		retriever = index
		if err := indexAllDailyContent(context.Background(), config.ContentDir, index); err != nil {
			return nil, fmt.Errorf("index daily content: %w", err)
		}
	}

	if config.DatabaseURL != "" {
		db, err := postgres.Open(context.Background(), config.DatabaseURL)
		if err != nil {
			return nil, fmt.Errorf("open memory database: %w", err)
		}

		postgresStore, err := postgres.NewStore(db)
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("create memory database store: %w", err)
		}
		if err := postgresStore.EnsureSchema(context.Background()); err != nil {
			_ = db.Close()
			return nil, err
		}

		store = postgresStore
		closeStore = db.Close
	}

	registry := tools.NewRegistry()

	translateTool, err := tools.NewTranslateTool(provider)
	if err != nil {
		return nil, fmt.Errorf("create translate tool: %w", err)
	}
	if err := registry.Register(translateTool); err != nil {
		return nil, fmt.Errorf("register translate tool: %w", err)
	}

	grammarTool, err := tools.NewGrammarTool(provider)
	if err != nil {
		return nil, fmt.Errorf("create grammar tool: %w", err)
	}
	if err := registry.Register(grammarTool); err != nil {
		return nil, fmt.Errorf("register grammar tool: %w", err)
	}

	vocabularyTool, err := tools.NewVocabularyTool(provider)
	if err != nil {
		return nil, fmt.Errorf("create vocabulary tool: %w", err)
	}
	if err := registry.Register(vocabularyTool); err != nil {
		return nil, fmt.Errorf("register vocabulary tool: %w", err)
	}

	reviewTool, err := tools.NewReviewTool(store)
	if err != nil {
		return nil, fmt.Errorf("create review tool: %w", err)
	}
	if err := registry.Register(reviewTool); err != nil {
		return nil, fmt.Errorf("register review tool: %w", err)
	}

	return NewRuntime(RuntimeConfig{
		Provider:          provider,
		Memory:            store,
		Tools:             registry,
		StructuredOutput:  true,
		ValidationRetries: 1,
		Retriever:         retriever,
		Close:             closeStore,
	})
}

func newEmbedder(config config.Config) (retrieval.Embedder, error) {
	switch config.EmbeddingProvider {
	case "openai":
		embedder, err := llm.NewOpenAIEmbedder(llm.OpenAIEmbedderConfig{APIKey: config.OpenAIAPIKey, Model: config.EmbeddingModel})
		if err != nil {
			return nil, fmt.Errorf("create OpenAI embedding provider: %w", err)
		}
		return embedder, nil
	case "ollama":
		embedder, err := llm.NewOllamaEmbedder(llm.OllamaEmbedderConfig{Model: config.EmbeddingModel, BaseURL: config.OllamaBaseURL})
		if err != nil {
			return nil, fmt.Errorf("create Ollama embedding provider: %w", err)
		}
		return embedder, nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", config.EmbeddingProvider)
	}
}

func indexAllDailyContent(ctx context.Context, contentDir string, index retrieval.Index) error {
	paths, err := filepath.Glob(filepath.Join(contentDir, "*.json"))
	if err != nil {
		return err
	}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var daily content.DailyContent
		if err := json.Unmarshal(data, &daily); err != nil {
			return err
		}
		if err := index.Upsert(ctx, retrieval.ChunksFromDocuments(retrieval.DocumentsFromDailyContent(daily))); err != nil {
			return err
		}
	}
	return nil
}
