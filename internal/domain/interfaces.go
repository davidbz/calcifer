package domain

import "context"

// Provider represents any LLM provider.
type Provider interface {
	// Complete sends a completion request and returns the full response.
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)

	// Stream sends a completion request and returns a stream of chunks.
	Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error)

	// Name returns the provider identifier.
	Name() string

	// IsModelSupported checks if the provider supports the given model.
	IsModelSupported(ctx context.Context, model string) bool
}

// ProviderRegistry manages available providers.
type ProviderRegistry interface {
	// Register adds a provider to the registry.
	Register(ctx context.Context, provider Provider) error

	// Get retrieves a provider by name.
	Get(ctx context.Context, providerName string) (Provider, error)

	// GetByModel retrieves a provider that supports the given model.
	GetByModel(ctx context.Context, model string) (Provider, error)

	// List returns all available providers.
	List(ctx context.Context) ([]string, error)
}
