package domain_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/davidbz/calcifer/internal/domain"
)

func TestStandardCostCalculator_Calculate(t *testing.T) {
	ctx := context.Background()
	registry := domain.NewInMemoryPricingRegistry()

	// Register test pricing
	err := registry.RegisterPricing(ctx, "test-model", domain.PricingConfig{
		InputCostPer1K:  0.01,
		OutputCostPer1K: 0.02,
	})
	require.NoError(t, err)

	calculator := domain.NewStandardCostCalculator(registry)

	tests := []struct {
		name         string
		model        string
		usage        domain.Usage
		expectedCost float64
		expectError  bool
	}{
		{
			name:  "calculate cost for known model",
			model: "test-model",
			usage: domain.Usage{
				PromptTokens:     1000,
				CompletionTokens: 500,
			},
			expectedCost: 0.02, // (1000/1000 * 0.01) + (500/1000 * 0.02)
			expectError:  false,
		},
		{
			name:  "unknown model returns zero cost",
			model: "unknown-model",
			usage: domain.Usage{
				PromptTokens:     1000,
				CompletionTokens: 500,
			},
			expectedCost: 0,
			expectError:  false,
		},
		{
			name:         "empty model returns error",
			model:        "",
			usage:        domain.Usage{},
			expectedCost: 0,
			expectError:  true,
		},
		{
			name:  "zero tokens returns zero cost",
			model: "test-model",
			usage: domain.Usage{
				PromptTokens:     0,
				CompletionTokens: 0,
			},
			expectedCost: 0,
			expectError:  false,
		},
		{
			name:  "partial tokens calculation",
			model: "test-model",
			usage: domain.Usage{
				PromptTokens:     250,
				CompletionTokens: 100,
			},
			expectedCost: 0.0045, // (250/1000 * 0.01) + (100/1000 * 0.02)
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCost, testErr := calculator.Calculate(ctx, tt.model, tt.usage)

			if tt.expectError {
				require.Error(t, testErr)
				return
			}

			require.NoError(t, testErr)
			require.InDelta(t, tt.expectedCost, testCost, 0.0001)
		})
	}
}

func TestInMemoryPricingRegistry_RegisterAndGet(t *testing.T) {
	ctx := context.Background()
	registry := domain.NewInMemoryPricingRegistry()

	t.Run("register and retrieve pricing", func(t *testing.T) {
		config := domain.PricingConfig{
			InputCostPer1K:  0.03,
			OutputCostPer1K: 0.06,
		}

		err := registry.RegisterPricing(ctx, "gpt-4", config)
		require.NoError(t, err)

		retrieved, err := registry.GetPricing(ctx, "gpt-4")
		require.NoError(t, err)
		require.InDelta(t, config.InputCostPer1K, retrieved.InputCostPer1K, 0.0001)
		require.InDelta(t, config.OutputCostPer1K, retrieved.OutputCostPer1K, 0.0001)
	})

	t.Run("get pricing for non-existent model returns error", func(t *testing.T) {
		_, err := registry.GetPricing(ctx, "non-existent-model")
		require.Error(t, err)
	})

	t.Run("register with empty model returns error", func(t *testing.T) {
		config := domain.PricingConfig{
			InputCostPer1K:  0.01,
			OutputCostPer1K: 0.02,
		}

		err := registry.RegisterPricing(ctx, "", config)
		require.Error(t, err)
	})

	t.Run("overwrite existing pricing", func(t *testing.T) {
		config1 := domain.PricingConfig{
			InputCostPer1K:  0.01,
			OutputCostPer1K: 0.02,
		}
		config2 := domain.PricingConfig{
			InputCostPer1K:  0.05,
			OutputCostPer1K: 0.10,
		}

		err := registry.RegisterPricing(ctx, "test-model", config1)
		require.NoError(t, err)

		err = registry.RegisterPricing(ctx, "test-model", config2)
		require.NoError(t, err)

		retrieved, err := registry.GetPricing(ctx, "test-model")
		require.NoError(t, err)
		require.InDelta(t, config2.InputCostPer1K, retrieved.InputCostPer1K, 0.0001)
		require.InDelta(t, config2.OutputCostPer1K, retrieved.OutputCostPer1K, 0.0001)
	})
}
