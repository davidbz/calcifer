package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

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

const (
	// shutdownTimeout is the maximum time to wait for graceful shutdown.
	shutdownTimeout = 30 * time.Second
)

// ErrProviderNotConfigured indicates that a provider is not configured and should be skipped.
var ErrProviderNotConfigured = errors.New("provider not configured")

func main() {
	container := buildContainer()
	ctx := context.Background()
	logger := observability.FromContext(ctx)

	// Start server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		err := container.Invoke(func(server *http.Server) error {
			return server.Start()
		})
		serverErr <- err
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		if err != nil {
			logger.Fatal("server failed to start", observability.Error(err))
		}
	case sig := <-quit:
		logger.Info("received shutdown signal, shutting down gracefully", observability.String("signal", sig.String()))
	}

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)

	err := container.Invoke(func(server *http.Server) error {
		return server.Shutdown(shutdownCtx)
	})
	cancel()

	if err != nil {
		logger.Error("server shutdown failed", observability.Error(err))
		os.Exit(1)
	}

	logger.Info("server shutdown complete")
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
	mustProvide(container, func(cfg *openai.Config) (*openai.Provider, error) {
		if cfg.APIKey == "" {
			return nil, ErrProviderNotConfigured
		}

		return openai.NewProvider(*cfg)
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
		ctx := context.Background()
		logger := observability.FromContext(ctx)
		logger.Fatal("failed to register providers", observability.Error(err))
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
		if err := openai.RegisterPricing(ctx, pricingReg); err != nil {
			return fmt.Errorf("failed to register OpenAI pricing: %w", err)
		}

		return nil
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
		ctx := context.Background()
		logger := observability.FromContext(ctx)
		logger.Fatal("failed to provide dependency", observability.Error(err))
	}
}

func mustInvoke(container *dig.Container, function any) {
	if err := container.Invoke(function); err != nil {
		ctx := context.Background()
		logger := observability.FromContext(ctx)
		logger.Fatal("failed to invoke function", observability.Error(err))
	}
}
