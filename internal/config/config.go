package config

import (
	"time"

	"github.com/caarlos0/env/v11"
	"github.com/joho/godotenv"
	"go.uber.org/dig"

	"github.com/davidbz/calcifer/internal/provider/openai"
)

// Config represents the gateway configuration.
type Config struct {
	Server ServerConfig
	CORS   CORSConfig
	OpenAI openai.Config
	Cache  CacheConfig
	Redis  RedisConfig
}

// ServerConfig contains HTTP server settings.
type ServerConfig struct {
	Port         int `env:"SERVER_PORT"          envDefault:"8080"`
	ReadTimeout  int `env:"SERVER_READ_TIMEOUT"  envDefault:"30"`
	WriteTimeout int `env:"SERVER_WRITE_TIMEOUT" envDefault:"30"`
}

// CORSConfig contains CORS policy settings.
type CORSConfig struct {
	AllowedOrigins   []string `env:"CORS_ALLOWED_ORIGINS"   envSeparator:"," envDefault:"*"`
	AllowedMethods   []string `env:"CORS_ALLOWED_METHODS"   envSeparator:"," envDefault:"GET,POST,PUT,DELETE,OPTIONS"`
	AllowedHeaders   []string `env:"CORS_ALLOWED_HEADERS"   envSeparator:"," envDefault:"Content-Type,Authorization"`
	AllowCredentials bool     `env:"CORS_ALLOW_CREDENTIALS"                  envDefault:"true"`
	MaxAge           int      `env:"CORS_MAX_AGE"                            envDefault:"86400"`
}

// CacheConfig contains semantic cache settings.
type CacheConfig struct {
	Enabled             bool          `env:"CACHE_ENABLED"              envDefault:"false"`
	SimilarityThreshold float64       `env:"CACHE_SIMILARITY_THRESHOLD" envDefault:"0.85"`
	TTL                 time.Duration `env:"CACHE_TTL"                  envDefault:"1h"`
	EmbeddingModel      string        `env:"CACHE_EMBEDDING_MODEL"      envDefault:"text-embedding-ada-002"`
	EmbeddingAPIKey     string        `env:"OPENAI_API_KEY"`
}

// RedisConfig contains Redis connection settings.
type RedisConfig struct {
	URL       string `env:"REDIS_URL"        envDefault:"redis://localhost:6379"`
	Password  string `env:"REDIS_PASSWORD"`
	DB        int    `env:"REDIS_DB"         envDefault:"0"`
	IndexName string `env:"REDIS_INDEX_NAME" envDefault:"calcifer_cache"`
}

// DepConfig is used for dependency injection with dig.
type DepConfig struct {
	dig.Out
	*ServerConfig
	*CORSConfig
	*openai.Config
	*CacheConfig
	*RedisConfig
}

// Load loads environment files and parses configuration.
func Load() *Config {
	for _, file := range []string{".env"} {
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
		&cfg.CORS,
		&cfg.OpenAI,
		&cfg.Cache,
		&cfg.Redis,
	}
}
