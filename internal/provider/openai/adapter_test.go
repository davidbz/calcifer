package openai_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/davidbz/calcifer/internal/provider/openai"
)

func TestNewProvider_Success(t *testing.T) {
	config := openai.Config{
		APIKey:     "test-api-key",
		BaseURL:    "https://api.openai.com/v1",
		Timeout:    60,
		MaxRetries: 3,
	}

	provider, err := openai.NewProvider(config)

	require.NoError(t, err)
	require.NotNil(t, provider)
	require.Equal(t, "openai", provider.Name())
}

func TestNewProvider_MissingAPIKey(t *testing.T) {
	config := openai.Config{
		APIKey:     "",
		BaseURL:    "https://api.openai.com/v1",
		Timeout:    60,
		MaxRetries: 3,
	}

	provider, err := openai.NewProvider(config)

	require.Error(t, err)
	require.Nil(t, provider)
	require.Contains(t, err.Error(), "OpenAI API key is required")
}

func TestProvider_Name(t *testing.T) {
	config := openai.Config{
		APIKey: "test-key",
	}
	provider, err := openai.NewProvider(config)
	require.NoError(t, err)

	require.Equal(t, "openai", provider.Name())
}

func TestProvider_IsModelSupported(t *testing.T) {
	config := openai.Config{
		APIKey: "test-key",
	}
	provider, err := openai.NewProvider(config)
	require.NoError(t, err)

	tests := []struct {
		name      string
		model     string
		supported bool
	}{
		{
			name:      "GPT-4 is supported",
			model:     "gpt-4",
			supported: true,
		},
		{
			name:      "GPT-4 Turbo is supported",
			model:     "gpt-4-turbo",
			supported: true,
		},
		{
			name:      "GPT-3.5 Turbo is supported",
			model:     "gpt-3.5-turbo",
			supported: true,
		},
		{
			name:      "Unknown model is not supported",
			model:     "unknown-model",
			supported: false,
		},
	}

	ctx := context.Background()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.IsModelSupported(ctx, tt.model)
			require.Equal(t, tt.supported, result)
		})
	}
}

func TestProvider_Complete_NilRequest(t *testing.T) {
	config := openai.Config{
		APIKey: "test-key",
	}
	provider, err := openai.NewProvider(config)
	require.NoError(t, err)

	ctx := context.Background()
	resp, err := provider.Complete(ctx, nil)

	require.Error(t, err)
	require.Nil(t, resp)
	require.Contains(t, err.Error(), "request cannot be nil")
}

func TestProvider_Stream_NilRequest(t *testing.T) {
	config := openai.Config{
		APIKey: "test-key",
	}
	provider, err := openai.NewProvider(config)
	require.NoError(t, err)

	ctx := context.Background()
	chunks, err := provider.Stream(ctx, nil)

	require.Error(t, err)
	require.Nil(t, chunks)
	require.Contains(t, err.Error(), "request cannot be nil")
}
