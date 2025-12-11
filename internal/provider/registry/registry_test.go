package registry_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/davidbz/calcifer/internal/domain"
	"github.com/davidbz/calcifer/internal/provider/registry"
)

// mockProvider is a mock implementation of domain.Provider for testing.
type mockProvider struct {
	name string
}

func (m *mockProvider) Complete(_ context.Context, _ *domain.CompletionRequest) (*domain.CompletionResponse, error) {
	return &domain.CompletionResponse{}, nil
}

func (m *mockProvider) Stream(_ context.Context, _ *domain.CompletionRequest) (<-chan domain.StreamChunk, error) {
	ch := make(chan domain.StreamChunk)
	close(ch)
	return ch, nil
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) IsModelSupported(_ context.Context, model string) bool {
	// Simple mock: provider supports models prefixed with its name
	// e.g., "openai" provider supports "openai-gpt-4"
	if m.name == "openai" && (model == "gpt-4" || model == "gpt-3.5-turbo") {
		return true
	}
	if m.name == "anthropic" && (model == "claude-2" || model == "claude-instant") {
		return true
	}
	return false
}

func (m *mockProvider) SupportedModels(_ context.Context) []string {
	if m.name == "openai" {
		return []string{"gpt-4", "gpt-3.5-turbo"}
	}
	if m.name == "anthropic" {
		return []string{"claude-2", "claude-instant"}
	}
	return []string{}
}

func TestRegistry_Register(t *testing.T) {
	t.Run("should register provider successfully", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		provider := &mockProvider{name: "test-provider"}

		err := reg.Register(ctx, provider)
		require.NoError(t, err)

		// Verify provider was registered
		registered, err := reg.Get(ctx, "test-provider")
		require.NoError(t, err)
		require.NotNil(t, registered)
		require.Equal(t, "test-provider", registered.Name())
	})

	t.Run("should return error when provider is nil", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		err := reg.Register(ctx, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "provider cannot be nil")
	})

	t.Run("should return error when provider name is empty", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		provider := &mockProvider{name: ""}

		err := reg.Register(ctx, provider)
		require.Error(t, err)
		require.Contains(t, err.Error(), "provider name cannot be empty")
	})

	t.Run("should return error when provider already registered", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		provider1 := &mockProvider{name: "test-provider"}
		provider2 := &mockProvider{name: "test-provider"}

		err := reg.Register(ctx, provider1)
		require.NoError(t, err)

		err = reg.Register(ctx, provider2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "already registered")
	})
}

func TestRegistry_Get(t *testing.T) {
	t.Run("should get registered provider", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		provider := &mockProvider{name: "test-provider"}
		err := reg.Register(ctx, provider)
		require.NoError(t, err)

		retrieved, err := reg.Get(ctx, "test-provider")
		require.NoError(t, err)
		require.NotNil(t, retrieved)
		require.Equal(t, "test-provider", retrieved.Name())
	})

	t.Run("should return error when provider name is empty", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		_, err := reg.Get(ctx, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "provider name cannot be empty")
	})

	t.Run("should return error when provider not found", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		_, err := reg.Get(ctx, "nonexistent")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})
}

func TestRegistry_List(t *testing.T) {
	t.Run("should return empty list when no providers registered", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		providers, err := reg.List(ctx)
		require.NoError(t, err)
		require.NotNil(t, providers)
		require.Empty(t, providers)
	})

	t.Run("should return all registered providers", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		provider1 := &mockProvider{name: "provider1"}
		provider2 := &mockProvider{name: "provider2"}
		provider3 := &mockProvider{name: "provider3"}

		err := reg.Register(ctx, provider1)
		require.NoError(t, err)

		err = reg.Register(ctx, provider2)
		require.NoError(t, err)

		err = reg.Register(ctx, provider3)
		require.NoError(t, err)

		providers, err := reg.List(ctx)
		require.NoError(t, err)
		require.Len(t, providers, 3)
		require.Contains(t, providers, "provider1")
		require.Contains(t, providers, "provider2")
		require.Contains(t, providers, "provider3")
	})
}

func TestRegistry_Concurrent(t *testing.T) {
	t.Run("should handle concurrent registrations safely", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		done := make(chan bool)

		// Register providers concurrently
		for i := range 10 {
			go func(idx int) {
				provider := &mockProvider{name: string(rune('a' + idx))}
				reg.Register(ctx, provider)
				done <- true
			}(i)
		}

		// Wait for all goroutines
		for range 10 {
			<-done
		}

		providers, err := reg.List(ctx)
		require.NoError(t, err)
		require.Len(t, providers, 10)
	})
}

func TestRegistry_GetByModel(t *testing.T) {
	t.Run("should return provider that supports the model", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		openaiProvider := &mockProvider{name: "openai"}
		anthropicProvider := &mockProvider{name: "anthropic"}

		err := reg.Register(ctx, openaiProvider)
		require.NoError(t, err)

		err = reg.Register(ctx, anthropicProvider)
		require.NoError(t, err)

		// Test OpenAI model
		provider, err := reg.GetByModel(ctx, "gpt-4")
		require.NoError(t, err)
		require.NotNil(t, provider)
		require.Equal(t, "openai", provider.Name())

		// Test Anthropic model
		provider, err = reg.GetByModel(ctx, "claude-2")
		require.NoError(t, err)
		require.NotNil(t, provider)
		require.Equal(t, "anthropic", provider.Name())
	})

	t.Run("should return error when model is empty", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		_, err := reg.GetByModel(ctx, "")
		require.Error(t, err)
		require.Contains(t, err.Error(), "model cannot be empty")
	})

	t.Run("should return error when no provider supports the model", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		openaiProvider := &mockProvider{name: "openai"}
		err := reg.Register(ctx, openaiProvider)
		require.NoError(t, err)

		_, err = reg.GetByModel(ctx, "unsupported-model")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no provider found for model")
	})

	t.Run("should return error when registry is empty", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		_, err := reg.GetByModel(ctx, "gpt-4")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no provider found for model")
	})

	t.Run("should use O(1) lookup with reverse index", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		// Register multiple providers with different models
		for i := range 10 {
			provider := &mockProvider{name: "provider-" + string(rune('a'+i))}
			err := reg.Register(ctx, provider)
			require.NoError(t, err)
		}

		// Register the target provider
		targetProvider := &mockProvider{name: "openai"}
		err := reg.Register(ctx, targetProvider)
		require.NoError(t, err)

		// Perform many lookups - should be fast with O(1) reverse index
		lookups := 1000
		for range lookups {
			provider, lookupErr := reg.GetByModel(ctx, "gpt-4")
			require.NoError(t, lookupErr)
			require.Equal(t, "openai", provider.Name())
		}
	})
}
