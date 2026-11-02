package retrieval

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type QdrantIndex struct {
	baseURL    string
	collection string
	embedder   Embedder
	client     *http.Client
	ready      bool
}

func NewQdrantIndex(baseURL, collection string, embedder Embedder, client *http.Client) (*QdrantIndex, error) {
	if strings.TrimSpace(baseURL) == "" || strings.TrimSpace(collection) == "" {
		return nil, fmt.Errorf("Qdrant URL and collection are required")
	}
	if embedder == nil {
		return nil, fmt.Errorf("Qdrant embedder is required")
	}
	if client == nil {
		client = http.DefaultClient
	}
	return &QdrantIndex{baseURL: strings.TrimRight(baseURL, "/"), collection: collection, embedder: embedder, client: client}, nil
}

func (q *QdrantIndex) Upsert(ctx context.Context, chunks []Chunk) error {
	if len(chunks) == 0 {
		return nil
	}
	texts := make([]string, len(chunks))
	for i := range chunks {
		texts[i] = chunks[i].Text
	}
	vectors, err := q.embedder.Embed(ctx, texts)
	if err != nil {
		return fmt.Errorf("embed Qdrant chunks: %w", err)
	}
	if len(vectors) != len(chunks) || len(vectors) == 0 || len(vectors[0]) == 0 {
		return fmt.Errorf("embedder returned an invalid vector batch")
	}
	if !q.ready {
		if err := q.ensureCollection(ctx, len(vectors[0])); err != nil {
			return err
		}
		q.ready = true
	}
	points := make([]map[string]any, 0, len(chunks))
	for i, chunk := range chunks {
		payload := map[string]any{"text": chunk.Text, "source": string(chunk.Source), "chunk_id": chunk.ID, "metadata": chunk.Metadata}
		points = append(points, map[string]any{"id": q.pointID(chunk.ID), "vector": vectors[i], "payload": payload})
	}
	return q.request(ctx, http.MethodPut, "/collections/"+url.PathEscape(q.collection)+"/points?wait=true", map[string]any{"points": points}, nil)
}

func (q *QdrantIndex) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	points := make([]string, 0, len(ids))
	for _, id := range ids {
		points = append(points, q.pointID(id))
	}
	return q.request(ctx, http.MethodPost, "/collections/"+url.PathEscape(q.collection)+"/points/delete?wait=true", map[string]any{"points": points}, nil)
}

func (q *QdrantIndex) Search(ctx context.Context, query string, limit int) ([]Result, error) {
	if limit < 0 {
		return nil, fmt.Errorf("search limit cannot be negative")
	}
	if limit == 0 {
		limit = 5
	}
	vectors, err := q.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, fmt.Errorf("embed Qdrant query: %w", err)
	}
	if len(vectors) != 1 {
		return nil, fmt.Errorf("embedder returned an invalid query vector")
	}
	var response struct {
		Result []struct {
			Score   float64 `json:"score"`
			Payload struct {
				Text     string            `json:"text"`
				Source   SourceKind        `json:"source"`
				ChunkID  string            `json:"chunk_id"`
				Metadata map[string]string `json:"metadata"`
			} `json:"payload"`
		} `json:"result"`
	}
	if err := q.request(ctx, http.MethodPost, "/collections/"+url.PathEscape(q.collection)+"/points/query", map[string]any{"query": vectors[0], "limit": limit, "with_payload": true}, &response); err != nil {
		return nil, err
	}
	results := make([]Result, 0, len(response.Result))
	for _, item := range response.Result {
		results = append(results, Result{Score: item.Score, Chunk: Chunk{ID: item.Payload.ChunkID, Text: item.Payload.Text, Source: item.Payload.Source, Metadata: item.Payload.Metadata}})
	}
	return results, nil
}

func (q *QdrantIndex) ensureCollection(ctx context.Context, size int) error {
	err := q.request(ctx, http.MethodPut, "/collections/"+url.PathEscape(q.collection), map[string]any{"vectors": map[string]any{"size": size, "distance": "Cosine"}}, nil)
	if err != nil && !strings.Contains(err.Error(), "409") {
		return fmt.Errorf("ensure Qdrant collection: %w", err)
	}
	return nil
}

func (q *QdrantIndex) request(ctx context.Context, method, path string, payload, result any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, method, q.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	res, err := q.client.Do(req)
	if err != nil {
		return fmt.Errorf("request Qdrant: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Qdrant returned %s", res.Status)
	}
	if result != nil {
		if err := json.NewDecoder(res.Body).Decode(result); err != nil {
			return fmt.Errorf("decode Qdrant response: %w", err)
		}
	}
	return nil
}

func (q *QdrantIndex) pointID(value string) string {
	sum := sha256.Sum256([]byte(value))
	raw := sum[:16]
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	hexValue := hex.EncodeToString(raw)
	return hexValue[0:8] + "-" + hexValue[8:12] + "-" + hexValue[12:16] + "-" + hexValue[16:20] + "-" + hexValue[20:32]
}
