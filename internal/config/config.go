package config

import (
	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"go.uber.org/dig"
)

// Config represents the gateway configuration.
type Config struct {
	Server ServerConfig
	OpenAI OpenAIConfig
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	Port         int `env:"SERVER_PORT"          envDefault:"8080"`
	ReadTimeout  int `env:"SERVER_READ_TIMEOUT"  envDefault:"30"`
	WriteTimeout int `env:"SERVER_WRITE_TIMEOUT" envDefault:"30"`
}

// OpenAIConfig contains OpenAI provider settings.
type OpenAIConfig struct {
	APIKey     string `env:"OPENAI_API_KEY"`
	BaseURL    string `env:"OPENAI_BASE_URL"    envDefault:"https://api.openai.com/v1"`
	Timeout    int    `env:"OPENAI_TIMEOUT"     envDefault:"60"`
	MaxRetries int    `env:"OPENAI_MAX_RETRIES" envDefault:"3"`
}

// DepConfig is used for dependency injection with dig.
type DepConfig struct {
	dig.Out
	*ServerConfig
	*OpenAIConfig
}

// Load loads environment files and parses configuration.
func Load() *Config {
	for _, file := range []string{".env", ".env.defaults", ".env.secrets"} {
		_ = godotenv.Load(file)
	}

	var cfg Config
	if err := env.Parse(&cfg); err != nil {
		panic(err)
	}

	return &cfg
}

// ParseDependenciesConfig returns pointers to sub-configs for dependency injection.
func ParseDependenciesConfig(cfg *Config) DepConfig {
	return DepConfig{
		dig.Out{},
		&cfg.Server,
		&cfg.OpenAI,
	}
}
