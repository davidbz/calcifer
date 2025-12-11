package openai

// Config holds configuration for OpenAI embedding generator.
type Config struct {
	APIKey string `env:"OPENAI_API_KEY"`
	Model  string `env:"CACHE_EMBEDDING_MODEL" envDefault:"text-embedding-ada-002"`
}
