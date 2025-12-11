package openai

import (
	"context"
	"fmt"

	"github.com/davidbz/calcifer/internal/domain"
)

const (
	// GPT-4 pricing per 1K tokens
	gpt4InputCostPer1K  = 0.03
	gpt4OutputCostPer1K = 0.06

	// GPT-4 Turbo pricing per 1K tokens
	gpt4TurboInputCostPer1K  = 0.01
	gpt4TurboOutputCostPer1K = 0.03

	// GPT-3.5 Turbo pricing per 1K tokens
	gpt35TurboInputCostPer1K  = 0.0005
	gpt35TurboOutputCostPer1K = 0.0015
)

// RegisterPricing registers OpenAI model pricing with the registry.
func RegisterPricing(ctx context.Context, registry domain.PricingRegistry) error {
	models := map[string]domain.PricingConfig{
		"gpt-4": {
			InputCostPer1K:  gpt4InputCostPer1K,
			OutputCostPer1K: gpt4OutputCostPer1K,
		},
		"gpt-4-turbo": {
			InputCostPer1K:  gpt4TurboInputCostPer1K,
			OutputCostPer1K: gpt4TurboOutputCostPer1K,
		},
		"gpt-3.5-turbo": {
			InputCostPer1K:  gpt35TurboInputCostPer1K,
			OutputCostPer1K: gpt35TurboOutputCostPer1K,
		},
	}

	for model, config := range models {
		if err := registry.RegisterPricing(ctx, model, config); err != nil {
			return fmt.Errorf("failed to register pricing for model %s: %w", model, err)
		}
	}

	return nil
}
