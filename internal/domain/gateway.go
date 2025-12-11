package domain

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/davidbz/calcifer/internal/observability"
)

// GatewayService orchestrates requests to providers.
type GatewayService struct {
	registry       ProviderRegistry
	costCalculator CostCalculator
	cache          SemanticCache
}

// NewGatewayService creates a new gateway service (DI constructor).
func NewGatewayService(registry ProviderRegistry, costCalculator CostCalculator, cache SemanticCache) *GatewayService {
	return &GatewayService{
		registry:       registry,
		costCalculator: costCalculator,
		cache:          cache,
	}
}

// Complete handles a completion request.
func (g *GatewayService) Complete(
	ctx context.Context,
	providerName string,
	req *CompletionRequest,
) (*CompletionResponse, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	if providerName == "" {
		return nil, errors.New("provider name cannot be empty")
	}

	// Route to appropriate provider.
	provider, err := g.registry.Get(ctx, providerName)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %w", err)
	}

	// Execute request.
	response, err := provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("completion failed: %w", err)
	}

	// Calculate cost in domain layer
	cost, _ := g.costCalculator.Calculate(ctx, response.Model, response.Usage)
	response.Usage.Cost = cost

	return response, nil
}

// Stream handles streaming completion requests.
func (g *GatewayService) Stream(
	ctx context.Context,
	providerName string,
	req *CompletionRequest,
) (<-chan StreamChunk, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	if providerName == "" {
		return nil, errors.New("provider name cannot be empty")
	}

	provider, err := g.registry.Get(ctx, providerName)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %w", err)
	}

	chunks, err := provider.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to stream from provider: %w", err)
	}
	return chunks, nil
}

// CompleteByModel handles a completion request with automatic provider routing.
func (g *GatewayService) CompleteByModel(
	ctx context.Context,
	req *CompletionRequest,
) (*CompletionResponse, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	if req.Model == "" {
		return nil, errors.New("model cannot be empty")
	}

	logger := observability.FromContext(ctx)

	// Check cache for non-streaming requests
	switch {
	case req.Stream:
		logger.Info("cache bypassed for streaming request")
	case g.cache == nil:
		logger.Info("cache is disabled (nil cache)")
	default:
		logger.Info("checking semantic cache",
			observability.String("model", req.Model),
			observability.Bool("cache_enabled", true))

		cached, cacheErr := g.cache.Get(ctx, req)
		if cacheErr != nil && !errors.Is(cacheErr, ErrCacheMiss) {
			logger.Warn("cache get failed, continuing without cache",
				observability.Error(cacheErr))
		}
		if cached != nil {
			logger.Info("cache HIT - returning cached response",
				observability.Float64("similarity_score", cached.SimilarityScore),
				observability.String("cached_model", cached.Response.Model))
			return cached.Response, nil
		}
		logger.Info("cache MISS - calling provider")
	}

	// Route to appropriate provider based on model.
	provider, err := g.registry.GetByModel(ctx, req.Model)
	if err != nil {
		return nil, fmt.Errorf("provider routing failed: %w", err)
	}

	// Execute request.
	response, err := provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("completion failed: %w", err)
	}

	// Calculate cost in domain layer
	cost, _ := g.costCalculator.Calculate(ctx, response.Model, response.Usage)
	response.Usage.Cost = cost

	// Store in cache (only for non-streaming requests)
	if !req.Stream && g.cache != nil {
		if setErr := g.cache.Set(ctx, req, response, 1*time.Hour); setErr != nil {
			logger.Warn("failed to store in cache",
				observability.Error(setErr))
		}
	}

	return response, nil
}

// StreamByModel handles streaming completion requests with automatic provider routing.
func (g *GatewayService) StreamByModel(
	ctx context.Context,
	req *CompletionRequest,
) (<-chan StreamChunk, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	if req.Model == "" {
		return nil, errors.New("model cannot be empty")
	}

	provider, err := g.registry.GetByModel(ctx, req.Model)
	if err != nil {
		return nil, fmt.Errorf("provider routing failed: %w", err)
	}

	chunks, err := provider.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to stream from provider: %w", err)
	}
	return chunks, nil
}
