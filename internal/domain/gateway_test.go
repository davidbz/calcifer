package domain_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/davidbz/calcifer/internal/domain"
	"github.com/stretchr/testify/require"
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

// mockEventBus is a mock implementation of EventPublisher for testing.
type mockEventBus struct {
	events []mockEvent
}

type mockEvent struct {
	eventType string
	data      map[string]interface{}
}

func newMockEventBus() *mockEventBus {
	return &mockEventBus{
		events: make([]mockEvent, 0),
	}
}

func (m *mockEventBus) Publish(_ context.Context, eventType string, data map[string]interface{}) {
	m.events = append(m.events, mockEvent{
		eventType: eventType,
		data:      data,
	})
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
		eventBus := newMockEventBus()
		gateway := domain.NewGatewayService(registry, eventBus)

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

		// Verify events were published
		require.Len(t, eventBus.events, 2)
		require.Equal(t, "request.started", eventBus.events[0].eventType)
		require.Equal(t, "request.completed", eventBus.events[1].eventType)
	})

	t.Run("should return error when request is nil", func(t *testing.T) {
		registry := newMockRegistry()
		eventBus := newMockEventBus()
		gateway := domain.NewGatewayService(registry, eventBus)

		ctx := context.Background()

		response, err := gateway.Complete(ctx, "test-provider", nil)

		require.Error(t, err)
		require.Nil(t, response)
		require.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("should return error when provider name is empty", func(t *testing.T) {
		registry := newMockRegistry()
		eventBus := newMockEventBus()
		gateway := domain.NewGatewayService(registry, eventBus)

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
		eventBus := newMockEventBus()
		gateway := domain.NewGatewayService(registry, eventBus)

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

	t.Run("should publish failed event when provider returns error", func(t *testing.T) {
		registry := newMockRegistry()
		eventBus := newMockEventBus()
		gateway := domain.NewGatewayService(registry, eventBus)

		provider := &mockProvider{
			name: "test-provider",
			completeFunc: func(_ context.Context, _ *domain.CompletionRequest) (*domain.CompletionResponse, error) {
				return nil, fmt.Errorf("provider error")
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

		// Verify failure event was published
		require.Len(t, eventBus.events, 2)
		require.Equal(t, "request.started", eventBus.events[0].eventType)
		require.Equal(t, "request.failed", eventBus.events[1].eventType)
	})
}

func TestGatewayService_Stream(t *testing.T) {
	t.Run("should stream request successfully", func(t *testing.T) {
		registry := newMockRegistry()
		eventBus := newMockEventBus()
		gateway := domain.NewGatewayService(registry, eventBus)

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

		// Verify stream started event was published
		require.Len(t, eventBus.events, 1)
		require.Equal(t, "stream.started", eventBus.events[0].eventType)
	})

	t.Run("should return error when request is nil", func(t *testing.T) {
		registry := newMockRegistry()
		eventBus := newMockEventBus()
		gateway := domain.NewGatewayService(registry, eventBus)

		ctx := context.Background()

		chunks, err := gateway.Stream(ctx, "test-provider", nil)

		require.Error(t, err)
		require.Nil(t, chunks)
		require.Contains(t, err.Error(), "request cannot be nil")
	})

	t.Run("should return error when provider name is empty", func(t *testing.T) {
		registry := newMockRegistry()
		eventBus := newMockEventBus()
		gateway := domain.NewGatewayService(registry, eventBus)

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
		eventBus := newMockEventBus()
		gateway := domain.NewGatewayService(registry, eventBus)

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
