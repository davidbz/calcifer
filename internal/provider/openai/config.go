package openai

// Config contains OpenAI provider configuration.
// Note: This mirrors config.OpenAIConfig but is defined here to avoid import cycles.
// All fields map to OpenAI SDK options:
//   - APIKey: Maps to option.WithAPIKey()
//   - BaseURL: Maps to option.WithBaseURL()
//   - Timeout: Maps to option.WithRequestTimeout() (in seconds)
//   - MaxRetries: Maps to option.WithMaxRetries()
type Config struct {
	APIKey     string
	BaseURL    string
	Timeout    int
	MaxRetries int
}
