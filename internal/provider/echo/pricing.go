package echo

import (
	"context"
	"fmt"

	"github.com/davidbz/calcifer/internal/domain"
)

const (
	echo4InputCostPer1K  = 0.0
	echo4OutputCostPer1K = 0.0
)

// RegisterPricing registers echo model pricing with the registry.
// Echo models have zero cost as they are for testing purposes only.
func RegisterPricing(ctx context.Context, registry domain.PricingRegistry) error {
	if err := registry.RegisterPricing(ctx, modelName, domain.PricingConfig{
		InputCostPer1K:  echo4InputCostPer1K,
		OutputCostPer1K: echo4OutputCostPer1K,
	}); err != nil {
		return fmt.Errorf("failed to register echo pricing: %w", err)
	}
	return nil
}
