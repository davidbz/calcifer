package openai

// Config contains OpenAI provider configuration.
// All fields map to OpenAI SDK options:
//   - APIKey: Maps to option.WithAPIKey()
//   - BaseURL: Maps to option.WithBaseURL()
//   - Timeout: Maps to option.WithRequestTimeout() (in seconds)
//   - MaxRetries: Maps to option.WithMaxRetries()
type Config struct {
	APIKey     string `env:"OPENAI_API_KEY"`
	BaseURL    string `env:"OPENAI_BASE_URL"    envDefault:"https://api.openai.com/v1"`
	Timeout    int    `env:"OPENAI_TIMEOUT"     envDefault:"60"`
	MaxRetries int    `env:"OPENAI_MAX_RETRIES" envDefault:"3"`
}
