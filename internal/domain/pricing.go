package domain

import "context"

// PricingConfig contains model pricing information.
type PricingConfig struct {
	InputCostPer1K  float64 // USD per 1K input tokens
	OutputCostPer1K float64 // USD per 1K output tokens
}

// CostCalculator calculates cost based on token usage.
type CostCalculator interface {
	// Calculate returns the total cost for a given model and usage.
	Calculate(ctx context.Context, model string, usage Usage) (float64, error)
}

// PricingRegistry maintains pricing information for models.
type PricingRegistry interface {
	// GetPricing returns pricing config for a model.
	GetPricing(ctx context.Context, model string) (PricingConfig, error)

	// RegisterPricing adds pricing for a model.
	RegisterPricing(ctx context.Context, model string, config PricingConfig) error
}
