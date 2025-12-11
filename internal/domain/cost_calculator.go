package domain

import (
	"context"
	"errors"
)

const tokensToPerK = 1000.0

// StandardCostCalculator implements standard token-based cost calculation.
type StandardCostCalculator struct {
	pricingRegistry PricingRegistry
}

// NewStandardCostCalculator creates a new cost calculator.
func NewStandardCostCalculator(registry PricingRegistry) *StandardCostCalculator {
	return &StandardCostCalculator{
		pricingRegistry: registry,
	}
}

// Calculate computes the total cost based on token usage and model pricing.
func (c *StandardCostCalculator) Calculate(
	ctx context.Context,
	model string,
	usage Usage,
) (float64, error) {
	if model == "" {
		return 0, errors.New("model cannot be empty")
	}

	pricing, err := c.pricingRegistry.GetPricing(ctx, model)
	if err != nil {
		// If pricing not found, return 0 cost (not an error for the request)
		//nolint:nilerr // Intentionally returning nil to allow requests with unknown pricing
		return 0, nil
	}

	inputCost := float64(usage.PromptTokens) / tokensToPerK * pricing.InputCostPer1K
	outputCost := float64(usage.CompletionTokens) / tokensToPerK * pricing.OutputCostPer1K
	totalCost := inputCost + outputCost

	return totalCost, nil
}
