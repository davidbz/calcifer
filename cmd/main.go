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
	"github.com/davidbz/calcifer/internal/http/middleware"
	"github.com/davidbz/calcifer/internal/observability"
	"github.com/davidbz/calcifer/internal/provider/echo"
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

	provideConfig(container)
	provideObservability(container)
	provideRegistries(container)
	provideCostCalculator(container)
	provideEcho(container)
	provideOpenAI(container)
	registerProviders(container)
	registerPricing(container)
	provideDomainServices(container)
	provideHTTPLayer(container)

	return container
}

func provideConfig(container *dig.Container) {
	mustProvide(container, config.Load)
	mustProvide(container, config.ParseDependenciesConfig)
}

func provideObservability(container *dig.Container) {
	mustProvide(container, observability.InitLogger)
}

func provideRegistries(container *dig.Container) {
	mustProvide(container, func() domain.ProviderRegistry {
		return registry.NewRegistry()
	})
	mustProvide(container, func() domain.PricingRegistry {
		return domain.NewInMemoryPricingRegistry()
	})
}

func provideCostCalculator(container *dig.Container) {
	mustProvide(container, func(reg domain.PricingRegistry) domain.CostCalculator {
		return domain.NewStandardCostCalculator(reg)
	})
}

func provideEcho(container *dig.Container) {
	mustProvide(container, echo.NewProvider)
}

func provideOpenAI(container *dig.Container) {
	mustProvide(container, func(cfg *config.Config) (*openai.Provider, error) {
		if cfg.OpenAI.APIKey == "" {
			return nil, ErrProviderNotConfigured
		}

		return openai.NewProvider(openai.Config{
			APIKey:     cfg.OpenAI.APIKey,
			BaseURL:    cfg.OpenAI.BaseURL,
			Timeout:    cfg.OpenAI.Timeout,
			MaxRetries: cfg.OpenAI.MaxRetries,
		})
	})
}

func registerProviders(container *dig.Container) {
	err := container.Invoke(func(
		reg domain.ProviderRegistry,
		echoProvider *echo.Provider,
		openaiProvider *openai.Provider,
	) error {
		ctx := context.Background()

		// Echo provider is always registered (no config needed)
		if err := reg.Register(ctx, echoProvider); err != nil {
			return fmt.Errorf("failed to register echo provider: %w", err)
		}

		if openaiProvider != nil {
			if err := reg.Register(ctx, openaiProvider); err != nil {
				return fmt.Errorf("failed to register OpenAI provider: %w", err)
			}
		}

		return nil
	})
	if err != nil && !errors.Is(err, ErrProviderNotConfigured) {
		log.Fatalf("Failed to register providers: %v", err)
	}
}

func registerPricing(container *dig.Container) {
	mustInvoke(container, func(pricingReg domain.PricingRegistry) error {
		ctx := context.Background()

		// Register echo pricing (zero cost)
		if err := echo.RegisterPricing(ctx, pricingReg); err != nil {
			return fmt.Errorf("failed to register echo pricing: %w", err)
		}

		// Register OpenAI pricing
		return openai.RegisterPricing(ctx, pricingReg)
	})
}

func provideDomainServices(container *dig.Container) {
	mustProvide(container, domain.NewGatewayService)
}

func provideHTTPLayer(container *dig.Container) {
	mustProvide(container, http.NewHandler)
	mustProvide(container, middleware.BuildMiddlewareChain)
	mustProvide(container, http.NewServer)
}

func mustProvide(container *dig.Container, constructor any) {
	if err := container.Provide(constructor); err != nil {
		log.Fatalf("Failed to provide dependency: %v", err)
	}
}

func mustInvoke(container *dig.Container, function any) {
	if err := container.Invoke(function); err != nil {
		log.Fatalf("Failed to invoke function: %v", err)
	}
}
