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

	// List returns all available providers.
	List(ctx context.Context) ([]string, error)
}

// EventPublisher publishes events for observability.
type EventPublisher interface {
	// Publish publishes an event with the given type and data.
	Publish(ctx context.Context, eventType string, data map[string]interface{})
}

// Router determines which provider to use for a request.
type Router interface {
	// Route selects a provider based on request criteria.
	Route(ctx context.Context, req *RouteRequest) (string, error)
}

// RouteRequest contains criteria for provider selection.
type RouteRequest struct {
	Model string
}
