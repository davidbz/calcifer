package domain

import (
	"context"
	"errors"
	"fmt"
)

// GatewayService orchestrates requests to providers.
type GatewayService struct {
	registry ProviderRegistry
}

// NewGatewayService creates a new gateway service (DI constructor).
func NewGatewayService(registry ProviderRegistry) *GatewayService {
	return &GatewayService{
		registry: registry,
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
