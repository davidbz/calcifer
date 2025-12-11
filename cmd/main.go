package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"go.uber.org/dig"

	"github.com/davidbz/calcifer/internal/config"
	"github.com/davidbz/calcifer/internal/domain"
	"github.com/davidbz/calcifer/internal/http"
	"github.com/davidbz/calcifer/internal/observability"
	"github.com/davidbz/calcifer/internal/provider/openai"
	"github.com/davidbz/calcifer/internal/provider/registry"
)

// ErrProviderNotConfigured indicates that a provider is not configured and should be skipped.
var ErrProviderNotConfigured = errors.New("provider not configured")

func main() {
	container := buildContainer()

	err := container.Invoke(func(server *http.Server) {
		if err := server.Start(); err != nil {
			log.Fatalf("Server failed to start: %v", err)
		}
	})
	if err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}
}

func buildContainer() *dig.Container {
	container := dig.New()

	// Configuration
	if err := container.Provide(config.Load); err != nil {
		log.Fatalf("Failed to provide config: %v", err)
	}
	if err := container.Provide(config.ParseDependenciesConfig); err != nil {
		log.Fatalf("Failed to provide config dependencies: %v", err)
	}

	// Observability
	if err := container.Provide(observability.InitLogger); err != nil {
		log.Fatalf("Failed to provide logger: %v", err)
	}

	// Provider Registry
	if err := container.Provide(func() domain.ProviderRegistry {
		return registry.NewRegistry()
	}); err != nil {
		log.Fatalf("Failed to provide registry: %v", err)
	}

	// OpenAI Provider
	if err := container.Provide(func(cfg *config.Config) (*openai.Provider, error) {
		if cfg.OpenAI.APIKey == "" {
			return nil, ErrProviderNotConfigured
		}

		return openai.NewProvider(openai.Config{
			APIKey:     cfg.OpenAI.APIKey,
			BaseURL:    cfg.OpenAI.BaseURL,
			Timeout:    cfg.OpenAI.Timeout,
			MaxRetries: cfg.OpenAI.MaxRetries,
		})
	}); err != nil {
		log.Fatalf("Failed to provide OpenAI provider: %v", err)
	}

	// Register providers with registry (invoked for side effects)
	if err := container.Invoke(func(
		reg domain.ProviderRegistry,
		openaiProvider *openai.Provider,
	) error {
		ctx := context.Background()

		// Register OpenAI if enabled
		if openaiProvider != nil {
			if err := reg.Register(ctx, openaiProvider); err != nil {
				return fmt.Errorf("failed to register OpenAI provider: %w", err)
			}
		}

		return nil
	}); err != nil {
		// Ignore ErrProviderNotConfigured as it's expected for optional providers
		if !errors.Is(err, ErrProviderNotConfigured) {
			log.Fatalf("Failed to register providers: %v", err)
		}
	}

	// Domain Services
	if err := container.Provide(domain.NewGatewayService); err != nil {
		log.Fatalf("Failed to provide gateway service: %v", err)
	}

	// HTTP Layer
	if err := container.Provide(http.NewHandler); err != nil {
		log.Fatalf("Failed to provide HTTP handler: %v", err)
	}
	if err := container.Provide(http.NewServer); err != nil {
		log.Fatalf("Failed to provide HTTP server: %v", err)
	}

	return container
}
