package domain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/davidbz/calcifer/internal/domain"
	"github.com/davidbz/calcifer/internal/mocks"
)

func TestGatewayService_Complete(t *testing.T) {
	t.Run("should complete request successfully", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		mockProvider := mocks.NewMockProvider(t)

		mockProvider.EXPECT().Complete(mock.Anything, mock.AnythingOfType("*domain.CompletionRequest")).Return(
			&domain.CompletionResponse{
				ID:       "test-id",
				Model:    "gpt-4",
				Provider: "test-provider",
				Content:  "test response",
				Usage: domain.Usage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
				FinishTime: time.Now(),
			}, nil)
		mockRegistry.EXPECT().Get(mock.Anything, "test-provider").Return(mockProvider, nil)
		mockCostCalc.EXPECT().Calculate(mock.Anything, "gpt-4", mock.AnythingOfType("domain.Usage")).Return(0.001, nil)

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "gpt-4",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
			Temperature: 0,
			MaxTokens:   0,
			Stream:      false,
			Metadata:    nil,
		}

		response, err := gateway.Complete(ctx, "test-provider", req)

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, "test-id", response.ID)
		require.Equal(t, "test-provider", response.Provider)
		require.Equal(t, "test response", response.Content)
		mockRegistry.AssertExpectations(t)
		mockCostCalc.AssertExpectations(t)
		mockProvider.AssertExpectations(t)
	})

	t.Run("should return error when request is nil", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()

		response, err := gateway.Complete(ctx, "test-provider", nil)

		require.Error(t, err)
		require.Nil(t, response)
		require.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("should return error when provider name is empty", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "gpt-4",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
			Temperature: 0,
			MaxTokens:   0,
			Stream:      false,
			Metadata:    nil,
		}

		response, err := gateway.Complete(ctx, "", req)

		require.Error(t, err)
		require.Nil(t, response)
		require.Contains(t, err.Error(), "provider name cannot be empty")
	})

	t.Run("should return error when provider not found", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)

		mockRegistry.EXPECT().
			Get(mock.Anything, "nonexistent").
			Return(nil, errors.New("provider not found: nonexistent"))

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "gpt-4",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
			Temperature: 0,
			MaxTokens:   0,
			Stream:      false,
			Metadata:    nil,
		}

		response, err := gateway.Complete(ctx, "nonexistent", req)

		require.Error(t, err)
		require.Nil(t, response)
		require.Contains(t, err.Error(), "provider not found")
		mockRegistry.AssertExpectations(t)
	})

	t.Run("should return error when provider returns error", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		mockProvider := mocks.NewMockProvider(t)

		mockProvider.EXPECT().
			Complete(mock.Anything, mock.AnythingOfType("*domain.CompletionRequest")).
			Return(nil, errors.New("provider error"))
		mockRegistry.EXPECT().Get(mock.Anything, "test-provider").Return(mockProvider, nil)

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "gpt-4",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
			Temperature: 0,
			MaxTokens:   0,
			Stream:      false,
			Metadata:    nil,
		}

		response, err := gateway.Complete(ctx, "test-provider", req)

		require.Error(t, err)
		require.Nil(t, response)
		require.Contains(t, err.Error(), "completion failed")
		mockRegistry.AssertExpectations(t)
		mockProvider.AssertExpectations(t)
	})
}

func TestGatewayService_Stream(t *testing.T) {
	t.Run("should stream request successfully", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		mockProvider := mocks.NewMockProvider(t)

		ch := make(chan domain.StreamChunk, 2)
		ch <- domain.StreamChunk{Delta: "test", Done: false}
		ch <- domain.StreamChunk{Done: true}
		close(ch)

		mockProvider.EXPECT().
			Stream(mock.Anything, mock.AnythingOfType("*domain.CompletionRequest")).
			Return((<-chan domain.StreamChunk)(ch), nil)
		mockRegistry.EXPECT().Get(mock.Anything, "test-provider").Return(mockProvider, nil)

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "gpt-4",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
			Temperature: 0,
			MaxTokens:   0,
			Stream:      true,
			Metadata:    nil,
		}

		chunks, err := gateway.Stream(ctx, "test-provider", req)

		require.NoError(t, err)
		require.NotNil(t, chunks)

		// Read chunks
		var receivedChunks []domain.StreamChunk
		for chunk := range chunks {
			receivedChunks = append(receivedChunks, chunk)
		}

		require.Len(t, receivedChunks, 2)
		require.Equal(t, "test", receivedChunks[0].Delta)
		require.False(t, receivedChunks[0].Done)
		require.True(t, receivedChunks[1].Done)
		mockRegistry.AssertExpectations(t)
		mockProvider.AssertExpectations(t)
	})

	t.Run("should return error when request is nil", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()

		chunks, err := gateway.Stream(ctx, "test-provider", nil)

		require.Error(t, err)
		require.Nil(t, chunks)
		require.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("should return error when provider name is empty", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model:       "gpt-4",
			Messages:    nil,
			Temperature: 0,
			MaxTokens:   0,
			Stream:      true,
			Metadata:    nil,
		}

		chunks, err := gateway.Stream(ctx, "", req)

		require.Error(t, err)
		require.Nil(t, chunks)
		require.Contains(t, err.Error(), "provider name cannot be empty")
	})

	t.Run("should return error when provider not found", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)

		mockRegistry.EXPECT().
			Get(mock.Anything, "nonexistent").
			Return(nil, errors.New("provider not found: nonexistent"))

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model:       "gpt-4",
			Messages:    nil,
			Temperature: 0,
			MaxTokens:   0,
			Stream:      true,
			Metadata:    nil,
		}

		chunks, err := gateway.Stream(ctx, "nonexistent", req)

		require.Error(t, err)
		require.Nil(t, chunks)
		require.Contains(t, err.Error(), "provider not found")
		mockRegistry.AssertExpectations(t)
	})
}

func TestGatewayService_CompleteByModel(t *testing.T) {
	t.Run("should complete request with automatic routing", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		mockProvider := mocks.NewMockProvider(t)

		mockRegistry.EXPECT().GetByModel(mock.Anything, "gpt-4").Return(mockProvider, nil)
		mockProvider.EXPECT().Complete(mock.Anything, mock.AnythingOfType("*domain.CompletionRequest")).Return(
			&domain.CompletionResponse{
				ID:       "test-id",
				Model:    "gpt-4",
				Provider: "openai",
				Content:  "test response",
				Usage: domain.Usage{
					PromptTokens:     10,
					CompletionTokens: 20,
					TotalTokens:      30,
				},
				FinishTime: time.Now(),
			}, nil)
		mockCostCalc.EXPECT().Calculate(mock.Anything, "gpt-4", mock.AnythingOfType("domain.Usage")).Return(0.001, nil)

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "gpt-4",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
			Temperature: 0.7,
			MaxTokens:   100,
			Stream:      false,
		}

		response, err := gateway.CompleteByModel(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, "test-id", response.ID)
		require.Equal(t, "openai", response.Provider)
		require.Equal(t, "test response", response.Content)
		mockRegistry.AssertExpectations(t)
		mockCostCalc.AssertExpectations(t)
		mockProvider.AssertExpectations(t)
	})

	t.Run("should return error when request is nil", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()

		response, err := gateway.CompleteByModel(ctx, nil)

		require.Error(t, err)
		require.Nil(t, response)
		require.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("should return error when model is empty", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
		}

		response, err := gateway.CompleteByModel(ctx, req)

		require.Error(t, err)
		require.Nil(t, response)
		require.Contains(t, err.Error(), "model cannot be empty")
	})

	t.Run("should return error when no provider supports model", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)

		mockRegistry.EXPECT().
			GetByModel(mock.Anything, "unsupported-model").
			Return(nil, errors.New("no provider supports model: unsupported-model"))

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "unsupported-model",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
		}

		response, err := gateway.CompleteByModel(ctx, req)

		require.Error(t, err)
		require.Nil(t, response)
		require.Contains(t, err.Error(), "provider routing failed")
		mockRegistry.AssertExpectations(t)
	})

	t.Run("should return error when provider fails", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		mockProvider := mocks.NewMockProvider(t)

		mockRegistry.EXPECT().GetByModel(mock.Anything, "gpt-4").Return(mockProvider, nil)
		mockProvider.EXPECT().
			Complete(mock.Anything, mock.AnythingOfType("*domain.CompletionRequest")).
			Return(nil, errors.New("provider error"))

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "gpt-4",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
		}

		response, err := gateway.CompleteByModel(ctx, req)

		require.Error(t, err)
		require.Nil(t, response)
		require.Contains(t, err.Error(), "completion failed")
		mockRegistry.AssertExpectations(t)
		mockProvider.AssertExpectations(t)
	})
}

func TestGatewayService_StreamByModel(t *testing.T) {
	t.Run("should stream request with automatic routing", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		mockProvider := mocks.NewMockProvider(t)

		ch := make(chan domain.StreamChunk, 2)
		ch <- domain.StreamChunk{Delta: "test", Done: false}
		ch <- domain.StreamChunk{Done: true}
		close(ch)

		mockRegistry.EXPECT().GetByModel(mock.Anything, "gpt-4").Return(mockProvider, nil)
		mockProvider.EXPECT().
			Stream(mock.Anything, mock.AnythingOfType("*domain.CompletionRequest")).
			Return((<-chan domain.StreamChunk)(ch), nil)

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "gpt-4",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
			Stream: true,
		}

		chunks, err := gateway.StreamByModel(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, chunks)

		// Read chunks
		var receivedChunks []domain.StreamChunk
		for chunk := range chunks {
			receivedChunks = append(receivedChunks, chunk)
		}

		require.Len(t, receivedChunks, 2)
		require.Equal(t, "test", receivedChunks[0].Delta)
		require.False(t, receivedChunks[0].Done)
		require.True(t, receivedChunks[1].Done)
		mockRegistry.AssertExpectations(t)
		mockProvider.AssertExpectations(t)
	})

	t.Run("should return error when request is nil", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()

		chunks, err := gateway.StreamByModel(ctx, nil)

		require.Error(t, err)
		require.Nil(t, chunks)
		require.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("should return error when model is empty", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
			Stream: true,
		}

		chunks, err := gateway.StreamByModel(ctx, req)

		require.Error(t, err)
		require.Nil(t, chunks)
		require.Contains(t, err.Error(), "model cannot be empty")
	})

	t.Run("should return error when no provider supports model", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)

		mockRegistry.EXPECT().
			GetByModel(mock.Anything, "unsupported-model").
			Return(nil, errors.New("no provider supports model: unsupported-model"))

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "unsupported-model",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
			Stream: true,
		}

		chunks, err := gateway.StreamByModel(ctx, req)

		require.Error(t, err)
		require.Nil(t, chunks)
		require.Contains(t, err.Error(), "provider routing failed")
		mockRegistry.AssertExpectations(t)
	})

	t.Run("should return error when provider stream fails", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		mockProvider := mocks.NewMockProvider(t)

		mockRegistry.EXPECT().GetByModel(mock.Anything, "gpt-4").Return(mockProvider, nil)
		mockProvider.EXPECT().
			Stream(mock.Anything, mock.AnythingOfType("*domain.CompletionRequest")).
			Return(nil, errors.New("stream error"))

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "gpt-4",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
			Stream: true,
		}

		chunks, err := gateway.StreamByModel(ctx, req)

		require.Error(t, err)
		require.Nil(t, chunks)
		require.Contains(t, err.Error(), "failed to stream from provider")
		mockRegistry.AssertExpectations(t)
		mockProvider.AssertExpectations(t)
	})
}

func TestGatewayService_CompleteByModel_WithCache(t *testing.T) {
	t.Run("should return cached response on cache hit", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		mockCache := mocks.NewMockSemanticCache(t)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "gpt-4",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
			Stream: false,
		}

		cachedResp := &domain.CompletionResponse{
			ID:       "cached-123",
			Model:    "gpt-4",
			Provider: "openai",
			Content:  "Cached response",
		}

		mockCache.EXPECT().
			Get(mock.Anything, req).
			Return(&domain.CachedResponse{Response: cachedResp}, nil)

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, mockCache)

		response, err := gateway.CompleteByModel(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, "cached-123", response.ID)
		require.Equal(t, "Cached response", response.Content)
		mockCache.AssertExpectations(t)
	})

	t.Run("should call provider on cache miss and store result", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		mockCache := mocks.NewMockSemanticCache(t)
		mockProvider := mocks.NewMockProvider(t)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "gpt-4",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
			Stream: false,
		}

		providerResp := &domain.CompletionResponse{
			ID:       "provider-123",
			Model:    "gpt-4",
			Provider: "openai",
			Content:  "Provider response",
			Usage: domain.Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		mockCache.EXPECT().
			Get(mock.Anything, req).
			Return(nil, nil)

		mockRegistry.EXPECT().
			GetByModel(mock.Anything, "gpt-4").
			Return(mockProvider, nil)

		mockProvider.EXPECT().
			Complete(mock.Anything, req).
			Return(providerResp, nil)

		mockCostCalc.EXPECT().
			Calculate(mock.Anything, "gpt-4", mock.AnythingOfType("domain.Usage")).
			Return(0.001, nil)

		mockCache.EXPECT().
			Set(mock.Anything, req, providerResp, mock.AnythingOfType("time.Duration")).
			Return(nil)

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, mockCache)

		response, err := gateway.CompleteByModel(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, "provider-123", response.ID)
		require.Equal(t, "Provider response", response.Content)

		mockRegistry.AssertExpectations(t)
		mockProvider.AssertExpectations(t)
		mockCostCalc.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})

	t.Run("should store non-streaming requests in cache", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		mockCache := mocks.NewMockSemanticCache(t)
		mockProvider := mocks.NewMockProvider(t)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "gpt-4",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
			Stream: false,
		}

		providerResp := &domain.CompletionResponse{
			ID:       "provider-123",
			Model:    "gpt-4",
			Provider: "openai",
			Content:  "Provider response",
			Usage: domain.Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		mockCache.EXPECT().
			Get(mock.Anything, req).
			Return(nil, domain.ErrCacheMiss)

		mockRegistry.EXPECT().
			GetByModel(mock.Anything, "gpt-4").
			Return(mockProvider, nil)

		mockProvider.EXPECT().
			Complete(mock.Anything, req).
			Return(providerResp, nil)

		mockCostCalc.EXPECT().
			Calculate(mock.Anything, "gpt-4", mock.AnythingOfType("domain.Usage")).
			Return(0.001, nil)

		mockCache.EXPECT().
			Set(mock.Anything, req, providerResp, mock.AnythingOfType("time.Duration")).
			Return(nil)

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, mockCache)

		response, err := gateway.CompleteByModel(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, "provider-123", response.ID)

		mockRegistry.AssertExpectations(t)
		mockProvider.AssertExpectations(t)
		mockCostCalc.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})

	t.Run("should continue on cache error", func(t *testing.T) {
		mockRegistry := mocks.NewMockProviderRegistry(t)
		mockCostCalc := mocks.NewMockCostCalculator(t)
		mockCache := mocks.NewMockSemanticCache(t)
		mockProvider := mocks.NewMockProvider(t)

		ctx := context.Background()
		req := &domain.CompletionRequest{
			Model: "gpt-4",
			Messages: []domain.Message{
				{Role: "user", Content: "Hello"},
			},
			Stream: false,
		}

		providerResp := &domain.CompletionResponse{
			ID:       "provider-123",
			Model:    "gpt-4",
			Provider: "openai",
			Content:  "Provider response",
			Usage: domain.Usage{
				PromptTokens:     10,
				CompletionTokens: 20,
				TotalTokens:      30,
			},
		}

		mockCache.EXPECT().
			Get(mock.Anything, req).
			Return(nil, errors.New("cache error"))

		mockRegistry.EXPECT().
			GetByModel(mock.Anything, "gpt-4").
			Return(mockProvider, nil)

		mockProvider.EXPECT().
			Complete(mock.Anything, req).
			Return(providerResp, nil)

		mockCostCalc.EXPECT().
			Calculate(mock.Anything, "gpt-4", mock.AnythingOfType("domain.Usage")).
			Return(0.001, nil)

		mockCache.EXPECT().
			Set(mock.Anything, req, providerResp, mock.AnythingOfType("time.Duration")).
			Return(nil)

		gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, mockCache)

		response, err := gateway.CompleteByModel(ctx, req)

		require.NoError(t, err)
		require.NotNil(t, response)
		require.Equal(t, "provider-123", response.ID)

		mockRegistry.AssertExpectations(t)
		mockProvider.AssertExpectations(t)
		mockCostCalc.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})
}
