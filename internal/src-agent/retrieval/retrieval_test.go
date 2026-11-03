package retrieval

import (
	"context"
	"testing"

	"github.com/meshestra/go-playground/jp/internal/src-agent/content"
)

func TestDailyContentDocumentsCarryProvenance(t *testing.T) {
	documents := DocumentsFromDailyContent(content.DailyContent{Date: "2026-11-01", Level: "N3", Theme: "운동", Words: []content.Word{{Word: "走る", Meaning: "달리다"}}, Grammar: content.Grammar{Pattern: "より"}})
	if len(documents) != 3 {
		t.Fatalf("expected overview, word, and grammar documents, got %d", len(documents))
	}
	if documents[1].Source != SourceVocabulary || documents[1].Metadata["date"] != "2026-11-01" {
		t.Fatalf("missing vocabulary provenance: %+v", documents[1])
	}
}

func TestLexicalIndexRanksMatchingChunks(t *testing.T) {
	index := NewLexicalIndex()
	if err := index.Upsert(context.Background(), []Chunk{{ID: "grammar", Text: "より 비교 기준", Source: SourceGrammar}, {ID: "food", Text: "식사와 수분", Source: SourceVocabulary}}); err != nil {
		t.Fatal(err)
	}
	results, err := index.Search(context.Background(), "より", 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Chunk.ID != "grammar" {
		t.Fatalf("unexpected search results: %+v", results)
	}
}
