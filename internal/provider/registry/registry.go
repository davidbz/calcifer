package registry

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/davidbz/calcifer/internal/domain"
)

// Registry implements the ProviderRegistry interface.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]domain.Provider
}

// NewRegistry creates a new provider registry.
func NewRegistry() *Registry {
	return &Registry{
		mu:        sync.RWMutex{},
		providers: make(map[string]domain.Provider),
	}
}

// Register adds a provider to the registry.
func (r *Registry) Register(_ context.Context, provider domain.Provider) error {
	if provider == nil {
		return errors.New("provider cannot be nil")
	}

	name := provider.Name()
	if name == "" {
		return errors.New("provider name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider %s already registered", name)
	}

	r.providers[name] = provider
	return nil
}

// Get retrieves a provider by name.
func (r *Registry) Get(_ context.Context, providerName string) (domain.Provider, error) {
	if providerName == "" {
		return nil, errors.New("provider name cannot be empty")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	provider, exists := r.providers[providerName]
	if !exists {
		return nil, fmt.Errorf("provider %s not found", providerName)
	}

	return provider, nil
}

// List returns all available providers.
func (r *Registry) List(_ context.Context) ([]string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.providers))
	for name := range r.providers {
		names = append(names, name)
	}

	return names, nil
}
