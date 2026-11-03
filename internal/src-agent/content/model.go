package content

import (
	"fmt"
	"strings"
)

type DailyContent struct {
	Date        string         `json:"date"`
	Level       string         `json:"level"`
	Theme       string         `json:"theme"`
	Situation   string         `json:"situation,omitempty"`
	Words       []Word         `json:"words"`
	Sentences   []Sentence     `json:"sentences"`
	Grammar     Grammar        `json:"grammar"`
	MomentsText string         `json:"moments_text"`
	Dialogue    []DialogueTurn `json:"dialogue,omitempty"`
	Culture     CultureContext `json:"culture,omitempty"`
	Visual      VisualContext  `json:"visual,omitempty"`
	Practice    []PracticeItem `json:"practice,omitempty"`
}

type DialogueTurn struct {
	Speaker     string `json:"speaker"`
	Japanese    string `json:"japanese"`
	Reading     string `json:"reading"`
	Translation string `json:"translation"`
}

type CultureContext struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Period  string `json:"period,omitempty"`
	Source  string `json:"source,omitempty"`
}

type VisualContext struct {
	Prompt string `json:"prompt"`
	Alt    string `json:"alt"`
	URL    string `json:"url,omitempty"`
}

type PracticeItem struct {
	Type   string `json:"type"`
	Prompt string `json:"prompt"`
	Answer string `json:"answer"`
}

type Word struct {
	Word               string `json:"word"`
	Reading            string `json:"reading"`
	Meaning            string `json:"meaning"`
	Example            string `json:"example"`
	ExampleTranslation string `json:"example_translation"`
}

type Sentence struct {
	Japanese    string `json:"japanese"`
	Reading     string `json:"reading"`
	Translation string `json:"translation"`
}

type Grammar struct {
	Pattern     string     `json:"pattern"`
	Meaning     string     `json:"meaning"`
	Explanation string     `json:"explanation"`
	Examples    []Sentence `json:"examples"`
}

func (d *DailyContent) Validate() error {
	if strings.TrimSpace(d.Date) == "" || strings.TrimSpace(d.Level) == "" {
		return fmt.Errorf("date and level are required")
	}

	if strings.TrimSpace(d.Theme) == "" {
		return fmt.Errorf("theme is required")
	}

	if len(d.Words) == 0 || len(d.Words) > 5 {
		return fmt.Errorf("words must contain between 1 and 5 items")
	}

	if len(d.Sentences) == 0 || len(d.Sentences) > 3 {
		return fmt.Errorf("sentences must contain between 1 and 3 items")
	}

	if strings.TrimSpace(d.Grammar.Pattern) == "" || len(d.Grammar.Examples) == 0 {
		return fmt.Errorf("grammar pattern and examples are required")
	}

	if strings.TrimSpace(d.MomentsText) == "" {
		return fmt.Errorf("moments_text is required")
	}

	return nil
}
