package routing_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/davidbz/calcifer/internal/domain"
	"github.com/davidbz/calcifer/internal/routing"
)

// mockRegistry is a mock implementation of ProviderRegistry for testing.
type mockRegistry struct {
	providers map[string]domain.Provider
}

func newMockRegistry() *mockRegistry {
	return &mockRegistry{
		providers: make(map[string]domain.Provider),
	}
}

func (m *mockRegistry) Register(_ context.Context, provider domain.Provider) error {
	m.providers[provider.Name()] = provider
	return nil
}

func (m *mockRegistry) Get(_ context.Context, providerName string) (domain.Provider, error) {
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

// mockProvider is a mock implementation of Provider for testing.
type mockProvider struct {
	name   string
	models map[string]struct{}
}

func (m *mockProvider) Complete(_ context.Context, _ *domain.CompletionRequest) (*domain.CompletionResponse, error) {
	return nil, nil
}

func (m *mockProvider) Stream(_ context.Context, _ *domain.CompletionRequest) (<-chan domain.StreamChunk, error) {
	return nil, nil
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) IsModelSupported(_ context.Context, model string) bool {
	_, supported := m.models[model]
	return supported
}

func TestRouter_Route(t *testing.T) {
	t.Run("should route to provider supporting the model", func(t *testing.T) {
		registry := newMockRegistry()
		router := routing.NewRouter(registry)

		provider := &mockProvider{
			name: "openai",
			models: map[string]struct{}{
				"gpt-4":         {},
				"gpt-3.5-turbo": {},
			},
		}
		registry.Register(context.Background(), provider)

		ctx := context.Background()
		req := &domain.RouteRequest{
			Model: "gpt-4",
		}

		providerName, err := router.Route(ctx, req)

		require.NoError(t, err)
		require.Equal(t, "openai", providerName)
	})

	t.Run("should return error when request is nil", func(t *testing.T) {
		registry := newMockRegistry()
		router := routing.NewRouter(registry)

		ctx := context.Background()

		providerName, err := router.Route(ctx, nil)

		require.Error(t, err)
		require.Empty(t, providerName)
		require.Contains(t, err.Error(), "route request cannot be nil")
	})

	t.Run("should return error when model is empty", func(t *testing.T) {
		registry := newMockRegistry()
		router := routing.NewRouter(registry)

		ctx := context.Background()
		req := &domain.RouteRequest{
			Model: "",
		}

		providerName, err := router.Route(ctx, req)

		require.Error(t, err)
		require.Empty(t, providerName)
		require.Contains(t, err.Error(), "model name is required")
	})

	t.Run("should return error when no provider supports the model", func(t *testing.T) {
		registry := newMockRegistry()
		router := routing.NewRouter(registry)

		provider := &mockProvider{
			name: "openai",
			models: map[string]struct{}{
				"gpt-4":         {},
				"gpt-3.5-turbo": {},
			},
		}
		registry.Register(context.Background(), provider)

		ctx := context.Background()
		req := &domain.RouteRequest{
			Model: "claude-3",
		}

		providerName, err := router.Route(ctx, req)

		require.Error(t, err)
		require.Empty(t, providerName)
		require.Contains(t, err.Error(), "no provider found for model")
	})

	t.Run("should select correct provider when multiple providers exist", func(t *testing.T) {
		registry := newMockRegistry()
		router := routing.NewRouter(registry)

		provider1 := &mockProvider{
			name: "openai",
			models: map[string]struct{}{
				"gpt-4":         {},
				"gpt-3.5-turbo": {},
			},
		}
		provider2 := &mockProvider{
			name: "anthropic",
			models: map[string]struct{}{
				"claude-3-opus":   {},
				"claude-3-sonnet": {},
			},
		}
		registry.Register(context.Background(), provider1)
		registry.Register(context.Background(), provider2)

		ctx := context.Background()
		req := &domain.RouteRequest{
			Model: "claude-3-opus",
		}

		providerName, err := router.Route(ctx, req)

		require.NoError(t, err)
		require.Equal(t, "anthropic", providerName)
	})

	t.Run("should return error when no providers available", func(t *testing.T) {
		registry := newMockRegistry()
		router := routing.NewRouter(registry)

		ctx := context.Background()
		req := &domain.RouteRequest{
			Model: "gpt-4",
		}

		providerName, err := router.Route(ctx, req)

		require.Error(t, err)
		require.Empty(t, providerName)
		require.Contains(t, err.Error(), "no providers available")
	})
}
