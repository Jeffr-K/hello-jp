package prompts

func NewDefaultRegistry() (*Registry, error) {
	renderer, err := NewRenderer()
	if err != nil {
		return nil, err
	}
	definitions := []PromptDefinition{
		{ID: "system.identity", Version: "1.0.0", TaskType: "system", Template: "identity.tmpl", Description: "Today Japanese identity and accuracy contract."},
		{ID: "system.behavior", Version: "1.0.0", TaskType: "system", Template: "behavior.tmpl", Description: "Korean-first response and uncertainty policy."},
		{ID: "tutor.teaching_policy", Version: "1.0.0", TaskType: "tutor", Template: "teaching_policy.tmpl", Description: "Progressive disclosure, scaffolding, correction and review policy."},
		{ID: "tutor.level_adaptation", Version: "1.0.0", TaskType: "tutor", Template: "level_adaptation.tmpl", Description: "JLPT-level adaptation guidance."},
		{ID: "tasks.translate", Version: "1.0.0", TaskType: "translate", Template: "translate.tmpl", SchemaName: "translation", Description: "Korean-Japanese translation."},
		{ID: "tasks.grammar", Version: "1.0.0", TaskType: "grammar", Template: "grammar.tmpl", SchemaName: "grammar", Description: "Japanese grammar analysis."},
		{ID: "tasks.vocabulary", Version: "1.0.0", TaskType: "vocabulary", Template: "vocabulary.tmpl", SchemaName: "vocabulary", Description: "Vocabulary teaching."},
		{ID: "tasks.conversation", Version: "1.0.0", TaskType: "conversation", Template: "conversation.tmpl", Description: "Guided conversation practice."},
		{ID: "tasks.correction", Version: "1.0.0", TaskType: "correction", Template: "correction.tmpl", Description: "Learner sentence correction."},
		{ID: "tasks.review", Version: "1.0.0", TaskType: "review", Template: "review.tmpl", Description: "Memory-grounded active recall."},
		{ID: "tasks.evaluation", Version: "1.0.0", TaskType: "evaluation", Template: "evaluation.tmpl", SchemaName: "evaluation", Description: "Answer evaluation."},
	}
	return NewRegistry(definitions, renderer)
}
