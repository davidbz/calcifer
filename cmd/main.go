package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	redisClient "github.com/redis/go-redis/v9"
	"go.uber.org/dig"

	redisCache "github.com/davidbz/calcifer/internal/cache/redis"
	"github.com/davidbz/calcifer/internal/config"
	"github.com/davidbz/calcifer/internal/domain"
	embeddingOpenAI "github.com/davidbz/calcifer/internal/embedding/openai"
	"github.com/davidbz/calcifer/internal/httpserver"
	"github.com/davidbz/calcifer/internal/httpserver/middleware"
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
		err := container.Invoke(func(server *httpserver.Server) error {
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

	err := container.Invoke(func(server *httpserver.Server) error {
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
	provideCache(container)
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

func provideCache(container *dig.Container) {
	// Provide Redis client (optional - returns nil if cache not enabled or connection fails)
	mustProvide(container, func(cfg *config.CacheConfig, redisCfg *config.RedisConfig) (*redisClient.Client, error) {
		if !cfg.Enabled {
			return nil, ErrProviderNotConfigured
		}

		opts, err := redisClient.ParseURL(redisCfg.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse redis URL: %w", err)
		}

		if redisCfg.Password != "" {
			opts.Password = redisCfg.Password
		}
		opts.DB = redisCfg.DB

		client := redisClient.NewClient(opts)

		pingCtx := context.Background()
		if pingErr := client.Ping(pingCtx).Err(); pingErr != nil {
			return nil, fmt.Errorf("redis connection failed: %w", pingErr)
		}

		return client, nil
	})

	// Provide SimilaritySearch (optional - returns nil if Redis client is nil)
	mustProvide(container, func(client *redisClient.Client, cfg *config.RedisConfig) (domain.SimilaritySearch, error) {
		if client == nil {
			return nil, ErrProviderNotConfigured
		}
		return redisCache.NewVectorSearch(client, cfg.IndexName)
	})

	// Provide EmbeddingGenerator (optional - returns nil if cache not enabled)
	mustProvide(container, func(cfg *config.CacheConfig) (domain.EmbeddingGenerator, error) {
		if !cfg.Enabled {
			return nil, ErrProviderNotConfigured
		}

		apiKey := os.Getenv("OPENAI_API_KEY")
		if apiKey == "" {
			return nil, errors.New("OPENAI_API_KEY required for cache embeddings")
		}

		return embeddingOpenAI.NewGenerator(embeddingOpenAI.Config{
			APIKey: apiKey,
			Model:  cfg.EmbeddingModel,
		})
	})

	// Provide SemanticCache (optional - returns nil if not enabled or dependencies unavailable)
	mustProvide(container, func(
		gen domain.EmbeddingGenerator,
		search domain.SimilaritySearch,
		cfg *config.CacheConfig,
	) domain.SemanticCache {
		if !cfg.Enabled || gen == nil || search == nil {
			return nil
		}
		return domain.NewSemanticCacheService(gen, search, cfg.SimilarityThreshold)
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
	mustProvide(container, httpserver.NewHandler)
	mustProvide(container, middleware.BuildMiddlewareChain)
	mustProvide(container, httpserver.NewServer)
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
