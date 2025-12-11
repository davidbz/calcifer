package registry_test

import (
	"context"
	"testing"

	"github.com/davidbz/calcifer/internal/domain"
	"github.com/davidbz/calcifer/internal/provider/registry"
	"github.com/stretchr/testify/require"
)

// mockProvider is a mock implementation of domain.Provider for testing.
type mockProvider struct {
	name string
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

func (m *mockProvider) IsModelSupported(_ context.Context, _ string) bool {
	return false
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
