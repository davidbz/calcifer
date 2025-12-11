package domain_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/davidbz/calcifer/internal/domain"
)

// mockRegistry is a mock implementation of ProviderRegistry for testing.
type mockRegistry struct {
	providers map[string]domain.Provider
	getError  error
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		providers: make(map[string]domain.Provider),
		getError:  nil,
	}
}

func (m *mockRegistry) Register(_ context.Context, provider domain.Provider) error {
	m.providers[provider.Name()] = provider
	return nil
}

func (m *mockRegistry) Get(_ context.Context, providerName string) (domain.Provider, error) {
	if m.getError != nil {
		return nil, m.getError
	}

	provider, exists := m.providers[providerName]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", providerName)
	}
	return provider, nil
}

func (m *mockRegistry) List(_ context.Context) ([]string, error) {
	names := make([]string, 0, len(m.providers))
	for name := range m.providers {
		names = append(names, name)
	}
	return names, nil
}

func (m *mockRegistry) GetByModel(ctx context.Context, model string) (domain.Provider, error) {
	for _, provider := range m.providers {
		if provider.IsModelSupported(ctx, model) {
			return provider, nil
		}
	}
	return nil, fmt.Errorf("no provider found for model: %s", model)
}

// mockProvider is a mock implementation of Provider for testing.
type mockProvider struct {
	name            string
	completeFunc    func(ctx context.Context, req *domain.CompletionRequest) (*domain.CompletionResponse, error)
	streamFunc      func(ctx context.Context, req *domain.CompletionRequest) (<-chan domain.StreamChunk, error)
	supportedModels map[string]struct{}
}

func (m *mockProvider) Complete(
	ctx context.Context,
	req *domain.CompletionRequest,
) (*domain.CompletionResponse, error) {
	if m.completeFunc != nil {
		return m.completeFunc(ctx, req)
	}
	return &domain.CompletionResponse{
		ID:       "test-id",
		Model:    req.Model,
		Provider: m.name,
		Content:  "test response",
		Usage: domain.Usage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
			Cost:             0.0,
		},
		FinishTime: time.Now(),
	}, nil
}

func (m *mockProvider) Stream(ctx context.Context, req *domain.CompletionRequest) (<-chan domain.StreamChunk, error) {
	if m.streamFunc != nil {
		return m.streamFunc(ctx, req)
	}
	chunks := make(chan domain.StreamChunk)
	go func() {
		defer close(chunks)
		chunks <- domain.StreamChunk{Delta: "test", Done: false, Error: nil}
		chunks <- domain.StreamChunk{Delta: "", Done: true, Error: nil}
	}()
	return chunks, nil
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) IsModelSupported(_ context.Context, model string) bool {
	if m.supportedModels == nil {
		return true
	}
	_, supported := m.supportedModels[model]
	return supported
}

func TestGatewayService_Complete(t *testing.T) {
	t.Run("should complete request successfully", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

		provider := &mockProvider{
			name:            "test-provider",
			completeFunc:    nil,
			streamFunc:      nil,
			supportedModels: nil,
		}
		registry.Register(context.Background(), provider)

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
	})

	t.Run("should return error when request is nil", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

		ctx := context.Background()

		response, err := gateway.Complete(ctx, "test-provider", nil)

		require.Error(t, err)
		require.Nil(t, response)
		require.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("should return error when provider name is empty", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

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
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

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
	})

	t.Run("should return error when provider returns error", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

		provider := &mockProvider{
			name: "test-provider",
			completeFunc: func(_ context.Context, _ *domain.CompletionRequest) (*domain.CompletionResponse, error) {
				return nil, errors.New("provider error")
			},
			streamFunc:      nil,
			supportedModels: nil,
		}
		registry.Register(context.Background(), provider)

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
	})
}

func TestGatewayService_Stream(t *testing.T) {
	t.Run("should stream request successfully", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

		provider := &mockProvider{
			name:            "test-provider",
			completeFunc:    nil,
			streamFunc:      nil,
			supportedModels: nil,
		}
		registry.Register(context.Background(), provider)

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
	})

	t.Run("should return error when request is nil", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

		ctx := context.Background()

		chunks, err := gateway.Stream(ctx, "test-provider", nil)

		require.Error(t, err)
		require.Nil(t, chunks)
		require.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("should return error when provider name is empty", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

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
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

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
	})
}

func TestGatewayService_CompleteByModel(t *testing.T) {
	t.Run("should complete request with automatic routing", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

		provider := &mockProvider{
			name:            "openai",
			completeFunc:    nil,
			streamFunc:      nil,
			supportedModels: map[string]struct{}{"gpt-4": {}},
		}
		registry.Register(context.Background(), provider)

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
	})

	t.Run("should return error when request is nil", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

		ctx := context.Background()

		response, err := gateway.CompleteByModel(ctx, nil)

		require.Error(t, err)
		require.Nil(t, response)
		require.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("should return error when model is empty", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

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
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

		provider := &mockProvider{
			name:            "openai",
			supportedModels: map[string]struct{}{"gpt-4": {}},
		}
		registry.Register(context.Background(), provider)

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
	})

	t.Run("should return error when provider fails", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

		provider := &mockProvider{
			name: "openai",
			completeFunc: func(_ context.Context, _ *domain.CompletionRequest) (*domain.CompletionResponse, error) {
				return nil, errors.New("provider error")
			},
			supportedModels: map[string]struct{}{"gpt-4": {}},
		}
		registry.Register(context.Background(), provider)

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
	})
}

func TestGatewayService_StreamByModel(t *testing.T) {
	t.Run("should stream request with automatic routing", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

		provider := &mockProvider{
			name:            "openai",
			completeFunc:    nil,
			streamFunc:      nil,
			supportedModels: map[string]struct{}{"gpt-4": {}},
		}
		registry.Register(context.Background(), provider)

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
	})

	t.Run("should return error when request is nil", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

		ctx := context.Background()

		chunks, err := gateway.StreamByModel(ctx, nil)

		require.Error(t, err)
		require.Nil(t, chunks)
		require.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("should return error when model is empty", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

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
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

		provider := &mockProvider{
			name:            "openai",
			supportedModels: map[string]struct{}{"gpt-4": {}},
		}
		registry.Register(context.Background(), provider)

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
	})

	t.Run("should return error when provider stream fails", func(t *testing.T) {
		registry := newMockRegistry()
		gateway := domain.NewGatewayService(registry)

		provider := &mockProvider{
			name: "openai",
			streamFunc: func(_ context.Context, _ *domain.CompletionRequest) (<-chan domain.StreamChunk, error) {
				return nil, errors.New("stream error")
			},
			supportedModels: map[string]struct{}{"gpt-4": {}},
		}
		registry.Register(context.Background(), provider)

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
	})
}
