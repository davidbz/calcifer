package routing

import (
	"context"
	"errors"
	"fmt"

	"github.com/davidbz/calcifer/internal/domain"
)

// SimpleRouter implements basic routing logic.
type SimpleRouter struct {
	registry domain.ProviderRegistry
}

// NewRouter creates a new router.
func NewRouter(registry domain.ProviderRegistry) *SimpleRouter {
	return &SimpleRouter{
		registry: registry,
	}
}

// Route selects a provider based on the model name.
func (r *SimpleRouter) Route(ctx context.Context, req *domain.RouteRequest) (string, error) {
	if req == nil {
		return "", errors.New("route request cannot be nil")
	}

	if req.Model == "" {
		return "", errors.New("model name is required")
	}

	providerNames, err := r.registry.List(ctx)
	if err != nil {
		return "", fmt.Errorf("failed to list providers: %w", err)
	}

	if len(providerNames) == 0 {
		return "", errors.New("no providers available")
	}

	for _, name := range providerNames {
		provider, getErr := r.registry.Get(ctx, name)
		if getErr != nil {
			continue
		}

		if provider.IsModelSupported(ctx, req.Model) {
			return name, nil
		}
	}

	return "", fmt.Errorf("no provider found for model: %s", req.Model)
}
