package openai

// SupportedModels returns the list of models supported by OpenAI provider.
func SupportedModels() []string {
	return []string{
		"gpt-4",
		"gpt-4-turbo",
		"gpt-4-turbo-preview",
		"gpt-3.5-turbo",
		"gpt-3.5-turbo-16k",
	}
}

// buildModelSet creates a map for O(1) lookup.
func buildModelSet(models []string) map[string]bool {
	set := make(map[string]bool, len(models))
	for _, model := range models {
		set[model] = true
	}
	return set
}
