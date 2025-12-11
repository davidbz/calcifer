package openai

// Config contains OpenAI provider configuration.
// Note: This mirrors config.OpenAIConfig but is defined here to avoid import cycles
type Config struct {
	APIKey     string
	BaseURL    string
	Timeout    int
	MaxRetries int
}
