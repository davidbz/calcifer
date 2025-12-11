package config_test

import (
	"os"
	"testing"

	"github.com/davidbz/calcifer/internal/config"
	"github.com/stretchr/testify/require"
)

func TestLoad(t *testing.T) {
	t.Run("should load config with defaults", func(t *testing.T) {
		// Clear environment
		os.Clearenv()

		cfg := config.Load()

		require.NotNil(t, cfg)

		// Verify defaults
		require.Equal(t, 8080, cfg.Server.Port)
		require.Equal(t, 30, cfg.Server.ReadTimeout)
		require.Equal(t, 30, cfg.Server.WriteTimeout)
		require.Equal(t, "https://api.openai.com/v1", cfg.OpenAI.BaseURL)
		require.Equal(t, 60, cfg.OpenAI.Timeout)
		require.Equal(t, 3, cfg.OpenAI.MaxRetries)
		require.Empty(t, cfg.OpenAI.APIKey)
	})

	t.Run("should load config from environment variables", func(t *testing.T) {
		// Set environment variables using t.Setenv for automatic cleanup
		t.Setenv("SERVER_PORT", "9000")
		t.Setenv("SERVER_READ_TIMEOUT", "60")
		t.Setenv("SERVER_WRITE_TIMEOUT", "60")
		t.Setenv("OPENAI_API_KEY", "sk-test-key")
		t.Setenv("OPENAI_BASE_URL", "https://test.openai.com")
		t.Setenv("OPENAI_TIMEOUT", "120")
		t.Setenv("OPENAI_MAX_RETRIES", "5")

		cfg := config.Load()

		require.NotNil(t, cfg)

		// Verify loaded values
		require.Equal(t, 9000, cfg.Server.Port)
		require.Equal(t, 60, cfg.Server.ReadTimeout)
		require.Equal(t, 60, cfg.Server.WriteTimeout)
		require.Equal(t, "sk-test-key", cfg.OpenAI.APIKey)
		require.Equal(t, "https://test.openai.com", cfg.OpenAI.BaseURL)
		require.Equal(t, 120, cfg.OpenAI.Timeout)
		require.Equal(t, 5, cfg.OpenAI.MaxRetries)
	})
}
