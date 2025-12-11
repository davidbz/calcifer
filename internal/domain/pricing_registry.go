package domain

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// InMemoryPricingRegistry stores pricing configs in memory.
type InMemoryPricingRegistry struct {
	mu      sync.RWMutex
	pricing map[string]PricingConfig
}

// NewInMemoryPricingRegistry creates a new in-memory pricing registry.
func NewInMemoryPricingRegistry() *InMemoryPricingRegistry {
	return &InMemoryPricingRegistry{
		mu:      sync.RWMutex{},
		pricing: make(map[string]PricingConfig),
	}
}

// GetPricing retrieves pricing for a model.
func (r *InMemoryPricingRegistry) GetPricing(
	_ context.Context,
	model string,
) (PricingConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	config, exists := r.pricing[model]
	if !exists {
		return PricingConfig{}, fmt.Errorf("pricing not found for model: %s", model)
	}

	return config, nil
}

// RegisterPricing adds pricing for a model.
func (r *InMemoryPricingRegistry) RegisterPricing(
	_ context.Context,
	model string,
	config PricingConfig,
) error {
	if model == "" {
		return errors.New("model cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.pricing[model] = config
	return nil
}
