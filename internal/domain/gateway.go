package domain

import (
	"context"
	"errors"
	"fmt"
)

// GatewayService orchestrates requests to providers.
type GatewayService struct {
	registry ProviderRegistry
	eventBus EventPublisher
}

// NewGatewayService creates a new gateway service (DI constructor).
func NewGatewayService(
	registry ProviderRegistry,
	eventBus EventPublisher,
) *GatewayService {
	return &GatewayService{
		registry: registry,
		eventBus: eventBus,
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

	// Publish event for observability.
	g.eventBus.Publish(ctx, "request.started", map[string]interface{}{
		"provider": providerName,
		"model":    req.Model,
	})

	// Execute request.
	response, err := provider.Complete(ctx, req)
	if err != nil {
		g.eventBus.Publish(ctx, "request.failed", map[string]interface{}{
			"provider": providerName,
			"error":    err.Error(),
		})
		return nil, fmt.Errorf("completion failed: %w", err)
	}

	// Publish success event.
	g.eventBus.Publish(ctx, "request.completed", map[string]interface{}{
		"provider": providerName,
		"tokens":   response.Usage.TotalTokens,
	})

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

	g.eventBus.Publish(ctx, "stream.started", map[string]interface{}{
		"provider": providerName,
		"model":    req.Model,
	})

	return provider.Stream(ctx, req)
}
