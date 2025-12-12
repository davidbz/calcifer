package domain

import (
	"context"
	"time"
)

// SemanticCache provides semantic caching operations using vector similarity.
type SemanticCache interface {
	// Get retrieves a cached response for a semantically similar request.
	Get(ctx context.Context, req *CompletionRequest) (*CachedResponse, error)

	// Set stores a response with its embedding in the cache.
	Set(ctx context.Context, req *CompletionRequest, resp *CompletionResponse, ttl time.Duration) error
}

// EmbeddingGenerator creates vector embeddings from text.
type EmbeddingGenerator interface {
	// Generate creates a vector embedding from text.
	Generate(ctx context.Context, text string) ([]float64, error)

	// Name returns the generator identifier.
	Name() string

	// Dimension returns the vector dimension.
	Dimension() int
}

// SimilaritySearch performs vector similarity search operations.
type SimilaritySearch interface {
	// Search finds similar vectors above the threshold.
	Search(ctx context.Context, embedding []float64, threshold float64, limit int) ([]*SearchResult, error)

	// Index stores a vector with associated data.
	Index(ctx context.Context, key string, embedding []float64, data []byte, ttl time.Duration) error
}

// CachedResponse represents a cached completion response with metadata.
type CachedResponse struct {
	Response        *CompletionResponse
	CachedAt        time.Time
	OriginalModel   string
	SimilarityScore float64
}

// SearchResult represents a vector search result.
type SearchResult struct {
	Key        string
	Similarity float64
	Data       []byte
	IndexedAt  time.Time
}
