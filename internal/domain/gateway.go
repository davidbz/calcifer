package domain

import (
	"context"
	"errors"
	"fmt"
	"strings"
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
	if g.cache != nil && !req.Stream {
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

	// Store in cache
	if g.cache != nil {
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

	logger := observability.FromContext(ctx)

	// Check cache even for streaming requests
	if g.cache != nil {
		logger.Info("checking semantic cache for streaming request",
			observability.String("model", req.Model))

		cached, cacheErr := g.cache.Get(ctx, req)
		if cacheErr != nil && !errors.Is(cacheErr, ErrCacheMiss) {
			logger.Warn("cache get failed, continuing without cache",
				observability.Error(cacheErr))
		}
		if cached != nil {
			logger.Info("cache HIT - streaming cached response",
				observability.Float64("similarity_score", cached.SimilarityScore),
				observability.String("cached_model", cached.Response.Model))
			return g.streamFromCache(cached.Response), nil
		}
		logger.Info("cache MISS - streaming from provider")
	}

	provider, err := g.registry.GetByModel(ctx, req.Model)
	if err != nil {
		return nil, fmt.Errorf("provider routing failed: %w", err)
	}

	chunks, err := provider.Stream(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("failed to stream from provider: %w", err)
	}

	// Wrap the stream to buffer content for caching
	if g.cache != nil {
		return g.cacheStreamWrapper(ctx, req, chunks), nil
	}

	return chunks, nil
}

// streamFromCache converts a cached response into a stream of chunks.
func (g *GatewayService) streamFromCache(response *CompletionResponse) <-chan StreamChunk {
	const (
		chunkSize     = 50 // characters per chunk
		streamDelayMs = 10 // milliseconds between chunks
	)

	out := make(chan StreamChunk)

	go func() {
		defer close(out)

		// Split content into chunks (simulate streaming)
		content := response.Content

		for i := 0; i < len(content); i += chunkSize {
			end := min(i+chunkSize, len(content))

			out <- StreamChunk{
				Delta: content[i:end],
				Done:  false,
				Error: nil,
			}

			// Small delay to simulate streaming
			time.Sleep(streamDelayMs * time.Millisecond)
		}

		// Send final done chunk
		out <- StreamChunk{
			Delta: "",
			Done:  true,
			Error: nil,
		}
	}()

	return out
}

// cacheStreamWrapper wraps a stream channel to buffer and cache the complete response.
func (g *GatewayService) cacheStreamWrapper(
	ctx context.Context,
	req *CompletionRequest,
	chunks <-chan StreamChunk,
) <-chan StreamChunk {
	out := make(chan StreamChunk)

	go func() {
		defer close(out)

		var contentBuilder strings.Builder
		var lastError error

		// Buffer all chunks and forward them
		for chunk := range chunks {
			out <- chunk

			if chunk.Error != nil {
				lastError = chunk.Error
			}

			if !chunk.Done {
				contentBuilder.WriteString(chunk.Delta)
			}
		}

		// Cache the complete response if no error occurred
		content := contentBuilder.String()
		if lastError == nil && content != "" {
			response := &CompletionResponse{
				ID:       fmt.Sprintf("stream-%d", time.Now().UnixNano()),
				Model:    req.Model,
				Provider: "cached-stream",
				Content:  content,
				Usage: Usage{
					PromptTokens:     0, // Not available from stream
					CompletionTokens: 0,
					TotalTokens:      0,
					Cost:             0,
				},
				FinishTime: time.Now(),
			}

			// Detach context for async cache operation (original request context is canceled)
			cacheCtx := observability.DetachContext(ctx)
			cacheLogger := observability.FromContext(cacheCtx)

			if setErr := g.cache.Set(cacheCtx, req, response, 1*time.Hour); setErr != nil {
				cacheLogger.Warn("failed to cache streamed response",
					observability.Error(setErr))
			} else {
				cacheLogger.Info("successfully cached streamed response",
					observability.Int("content_length", len(content)))
			}
		}
	}()

	return out
}
