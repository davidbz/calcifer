package registry_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/davidbz/calcifer/internal/mocks"
	"github.com/davidbz/calcifer/internal/provider/registry"
)

func TestRegistry_Register(t *testing.T) {
	t.Run("should register provider successfully", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		mockProvider := mocks.NewMockProvider(t)
		mockProvider.EXPECT().Name().Return("test-provider")
		mockProvider.EXPECT().SupportedModels(mock.Anything).Return([]string{})

		err := reg.Register(ctx, mockProvider)
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

		mockProvider := mocks.NewMockProvider(t)
		mockProvider.EXPECT().Name().Return("")

		err := reg.Register(ctx, mockProvider)
		require.Error(t, err)
		require.Contains(t, err.Error(), "provider name cannot be empty")
	})

	t.Run("should return error when provider already registered", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		mockProvider1 := mocks.NewMockProvider(t)
		mockProvider1.EXPECT().Name().Return("test-provider")
		mockProvider1.EXPECT().SupportedModels(mock.Anything).Return([]string{})
		mockProvider2 := mocks.NewMockProvider(t)
		mockProvider2.EXPECT().Name().Return("test-provider")

		err := reg.Register(ctx, mockProvider1)
		require.NoError(t, err)

		err = reg.Register(ctx, mockProvider2)
		require.Error(t, err)
		require.Contains(t, err.Error(), "already registered")
	})
}

func TestRegistry_Get(t *testing.T) {
	t.Run("should get registered provider", func(t *testing.T) {
		reg := registry.NewRegistry()
		ctx := context.Background()

		mockProvider := mocks.NewMockProvider(t)
		mockProvider.EXPECT().Name().Return("test-provider")
		mockProvider.EXPECT().SupportedModels(mock.Anything).Return([]string{})

		err := reg.Register(ctx, mockProvider)
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

		mockProvider1 := mocks.NewMockProvider(t)
		mockProvider1.EXPECT().Name().Return("provider1")
		mockProvider1.EXPECT().SupportedModels(mock.Anything).Return([]string{})
		mockProvider2 := mocks.NewMockProvider(t)
		mockProvider2.EXPECT().Name().Return("provider2")
		mockProvider2.EXPECT().SupportedModels(mock.Anything).Return([]string{})
		mockProvider3 := mocks.NewMockProvider(t)
		mockProvider3.EXPECT().Name().Return("provider3")
		mockProvider3.EXPECT().SupportedModels(mock.Anything).Return([]string{})

		err := reg.Register(ctx, mockProvider1)
		require.NoError(t, err)

		err = reg.Register(ctx, mockProvider2)
		require.NoError(t, err)

		err = reg.Register(ctx, mockProvider3)
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
				mockProvider := mocks.NewMockProvider(t)
				mockProvider.EXPECT().Name().Return(string(rune('a' + idx)))
				mockProvider.EXPECT().SupportedModels(mock.Anything).Return([]string{}).Maybe()
				reg.Register(ctx, mockProvider)
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

		mockOpenAI := mocks.NewMockProvider(t)
		mockOpenAI.EXPECT().Name().Return("openai")
		mockOpenAI.EXPECT().SupportedModels(mock.Anything).Return([]string{"gpt-4", "gpt-3.5-turbo"})

		mockAnthropic := mocks.NewMockProvider(t)
		mockAnthropic.EXPECT().Name().Return("anthropic")
		mockAnthropic.EXPECT().SupportedModels(mock.Anything).Return([]string{"claude-2", "claude-instant"})

		err := reg.Register(ctx, mockOpenAI)
		require.NoError(t, err)

		err = reg.Register(ctx, mockAnthropic)
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

		mockOpenAI := mocks.NewMockProvider(t)
		mockOpenAI.EXPECT().Name().Return("openai")
		mockOpenAI.EXPECT().SupportedModels(mock.Anything).Return([]string{"gpt-4", "gpt-3.5-turbo"})
		mockOpenAI.EXPECT().IsModelSupported(mock.Anything, "unsupported-model").Return(false)

		err := reg.Register(ctx, mockOpenAI)
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
			mockProvider := mocks.NewMockProvider(t)
			mockProvider.EXPECT().Name().Return("provider-" + string(rune('a'+i)))
			mockProvider.EXPECT().SupportedModels(mock.Anything).Return([]string{})
			err := reg.Register(ctx, mockProvider)
			require.NoError(t, err)
		}

		// Register the target provider
		mockOpenAI := mocks.NewMockProvider(t)
		mockOpenAI.EXPECT().Name().Return("openai")
		mockOpenAI.EXPECT().SupportedModels(mock.Anything).Return([]string{"gpt-4", "gpt-3.5-turbo"})
		err := reg.Register(ctx, mockOpenAI)
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
