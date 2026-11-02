package retrieval

import (
	"fmt"

	"github.com/meshestra/go-playground/jp/internal/content"
)

func DocumentsFromDailyContent(daily content.DailyContent) []Document {
	documents := []Document{{
		ID: fmt.Sprintf("daily:%s:overview", daily.Date), Source: SourceDailyContent,
		Text:     fmt.Sprintf("%s %s %s", daily.Theme, daily.Level, daily.MomentsText),
		Metadata: map[string]string{"date": daily.Date, "level": daily.Level, "theme": daily.Theme},
	}}
	for index, word := range daily.Words {
		documents = append(documents, Document{ID: fmt.Sprintf("daily:%s:word:%d", daily.Date, index), Source: SourceVocabulary, Text: fmt.Sprintf("%s %s %s %s %s", word.Word, word.Reading, word.Meaning, word.Example, word.ExampleTranslation), Metadata: map[string]string{"date": daily.Date, "level": daily.Level, "theme": daily.Theme}})
	}
	for index, sentence := range daily.Sentences {
		documents = append(documents, Document{ID: fmt.Sprintf("daily:%s:sentence:%d", daily.Date, index), Source: SourceDailyContent, Text: fmt.Sprintf("%s %s %s", sentence.Japanese, sentence.Reading, sentence.Translation), Metadata: map[string]string{"date": daily.Date, "level": daily.Level, "theme": daily.Theme}})
	}
	documents = append(documents, Document{ID: fmt.Sprintf("daily:%s:grammar", daily.Date), Source: SourceGrammar, Text: fmt.Sprintf("%s %s %s %s", daily.Grammar.Pattern, daily.Grammar.Meaning, daily.Grammar.Explanation, daily.Grammar.Examples), Metadata: map[string]string{"date": daily.Date, "level": daily.Level, "theme": daily.Theme}})
	for index, turn := range daily.Dialogue {
		documents = append(documents, Document{ID: fmt.Sprintf("daily:%s:dialogue:%d", daily.Date, index), Source: SourceConversation, Text: fmt.Sprintf("%s %s %s %s", turn.Speaker, turn.Japanese, turn.Reading, turn.Translation), Metadata: map[string]string{"date": daily.Date, "level": daily.Level, "theme": daily.Theme, "situation": daily.Situation}})
	}
	if daily.Culture.Title != "" || daily.Culture.Summary != "" {
		documents = append(documents, Document{ID: fmt.Sprintf("daily:%s:culture", daily.Date), Source: SourceDailyContent, Text: fmt.Sprintf("%s %s %s %s", daily.Culture.Title, daily.Culture.Summary, daily.Culture.Period, daily.Culture.Source), Metadata: map[string]string{"date": daily.Date, "level": daily.Level, "theme": daily.Theme}})
	}
	for index, item := range daily.Practice {
		documents = append(documents, Document{ID: fmt.Sprintf("daily:%s:practice:%d", daily.Date, index), Source: SourceConversation, Text: fmt.Sprintf("%s %s %s", item.Type, item.Prompt, item.Answer), Metadata: map[string]string{"date": daily.Date, "level": daily.Level, "theme": daily.Theme}})
	}
	return documents
}

func ChunksFromDocuments(documents []Document) []Chunk {
	chunks := make([]Chunk, 0, len(documents))
	for _, document := range documents {
		chunks = append(chunks, Chunk{ID: document.ID, Text: document.Text, Source: document.Source, Metadata: document.Metadata})
	}
	return chunks
}
