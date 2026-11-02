// Package retrieval defines the RAG boundary. It is deliberately independent
// of a particular vector database or embedding vendor.
package retrieval

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unicode"
)

type SourceKind string

const (
	SourceDailyContent SourceKind = "daily_content"
	SourceGrammar      SourceKind = "grammar"
	SourceVocabulary   SourceKind = "vocabulary"
	SourceConversation SourceKind = "conversation"
)

type Document struct {
	ID       string
	Text     string
	Source   SourceKind
	Metadata map[string]string
}

type Chunk struct {
	ID       string
	Text     string
	Source   SourceKind
	Metadata map[string]string
}

type Result struct {
	Chunk Chunk
	Score float64
}

type Index interface {
	Upsert(context.Context, []Chunk) error
	Delete(context.Context, []string) error
	Search(context.Context, string, int) ([]Result, error)
}

type Embedder interface {
	Embed(context.Context, []string) ([][]float32, error)
}

// LexicalIndex is a deterministic local implementation for development and
// regression tests. It can later be replaced with a pgvector or hosted index.
type LexicalIndex struct {
	mu     sync.RWMutex
	chunks map[string]Chunk
}

func NewLexicalIndex() *LexicalIndex { return &LexicalIndex{chunks: make(map[string]Chunk)} }

func (i *LexicalIndex) Upsert(ctx context.Context, chunks []Chunk) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, chunk := range chunks {
		i.chunks[chunk.ID] = chunk
	}
	return nil
}

func (i *LexicalIndex) Delete(ctx context.Context, ids []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	for _, id := range ids {
		delete(i.chunks, id)
	}
	return nil
}

func (i *LexicalIndex) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if limit < 0 {
		return nil, fmt.Errorf("search limit cannot be negative")
	}
	terms := tokenize(query)
	i.mu.RLock()
	defer i.mu.RUnlock()
	results := make([]Result, 0)
	for _, chunk := range i.chunks {
		score := overlap(terms, tokenize(chunk.Text))
		if score > 0 {
			results = append(results, Result{Chunk: chunk, Score: score})
		}
	}
	sort.Slice(results, func(a, b int) bool {
		if results[a].Score == results[b].Score {
			return results[a].Chunk.ID < results[b].Chunk.ID
		}
		return results[a].Score > results[b].Score
	})
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func tokenize(value string) map[string]struct{} {
	result := make(map[string]struct{})
	for _, field := range strings.FieldsFunc(strings.ToLower(value), func(r rune) bool { return unicode.IsSpace(r) || strings.ContainsRune(".,!?()[]{}:;·。、「」", r) }) {
		if field != "" {
			result[field] = struct{}{}
		}
	}
	return result
}

func overlap(left, right map[string]struct{}) float64 {
	if len(left) == 0 {
		return 0
	}
	matches := 0
	for term := range left {
		if _, ok := right[term]; ok {
			matches++
		}
	}
	return float64(matches) / float64(len(left))
}
