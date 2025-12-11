package domain

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/davidbz/calcifer/internal/observability"
)

// ErrCacheMiss indicates no cached entry was found.
var ErrCacheMiss = errors.New("cache miss")

// SemanticCacheService implements semantic caching using embeddings and vector search.
type SemanticCacheService struct {
	embeddingGen     EmbeddingGenerator
	similaritySearch SimilaritySearch
	threshold        float64
}

// NewSemanticCacheService creates a new semantic cache service.
func NewSemanticCacheService(
	embeddingGen EmbeddingGenerator,
	similaritySearch SimilaritySearch,
	threshold float64,
) *SemanticCacheService {
	return &SemanticCacheService{
		embeddingGen:     embeddingGen,
		similaritySearch: similaritySearch,
		threshold:        threshold,
	}
}

// Get retrieves a cached response for a semantically similar request.
func (s *SemanticCacheService) Get(ctx context.Context, req *CompletionRequest) (*CachedResponse, error) {
	logger := observability.FromContext(ctx)
	logger.Info("semantic cache Get started")

	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	queryText := s.buildQueryText(req)
	logger.Info("built query text for embedding",
		observability.Int("query_length", len(queryText)))

	embedding, err := s.embeddingGen.Generate(ctx, queryText)
	if err != nil {
		logger.Error("failed to generate embedding",
			observability.Error(err))
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}
	logger.Info("embedding generated",
		observability.Int("embedding_dimension", len(embedding)))

	results, err := s.similaritySearch.Search(ctx, embedding, s.threshold, 1)
	if err != nil {
		logger.Error("similarity search failed",
			observability.Error(err),
			observability.Float64("threshold", s.threshold))
		return nil, fmt.Errorf("failed to search similar vectors: %w", err)
	}

	if len(results) == 0 {
		logger.Info("no similar results found in cache",
			observability.Float64("threshold", s.threshold))
		return nil, ErrCacheMiss
	}

	logger.Info("found similar cached entry",
		observability.Float64("similarity", results[0].Similarity),
		observability.String("cache_key", results[0].Key))

	//nolint:exhaustruct // Response field is populated via json.Unmarshal below
	cached := &CachedResponse{
		SimilarityScore: results[0].Similarity,
		CachedAt:        results[0].IndexedAt,
		OriginalModel:   req.Model,
	}

	if unmarshalErr := json.Unmarshal(results[0].Data, &cached.Response); unmarshalErr != nil {
		logger.Error("failed to unmarshal cached response",
			observability.Error(unmarshalErr))
		return nil, fmt.Errorf("failed to unmarshal cached response: %w", unmarshalErr)
	}

	return cached, nil
}

// Set stores a response with its embedding in the cache.
func (s *SemanticCacheService) Set(
	ctx context.Context,
	req *CompletionRequest,
	resp *CompletionResponse,
	ttl time.Duration,
) error {
	logger := observability.FromContext(ctx)
	logger.Info("semantic cache Set started",
		observability.Duration("ttl", ttl))

	if req == nil {
		return errors.New("request cannot be nil")
	}

	if resp == nil {
		return errors.New("response cannot be nil")
	}

	queryText := s.buildQueryText(req)

	embedding, err := s.embeddingGen.Generate(ctx, queryText)
	if err != nil {
		logger.Error("failed to generate embedding for cache storage",
			observability.Error(err))
		return fmt.Errorf("failed to generate embedding: %w", err)
	}
	logger.Info("embedding generated for storage",
		observability.Int("embedding_dimension", len(embedding)))

	data, err := json.Marshal(resp)
	if err != nil {
		logger.Error("failed to marshal response",
			observability.Error(err))
		return fmt.Errorf("failed to marshal response: %w", err)
	}

	cacheKey := s.generateCacheKey(queryText)
	logger.Info("indexing response in cache",
		observability.String("cache_key", cacheKey),
		observability.Int("data_size", len(data)))

	if indexErr := s.similaritySearch.Index(ctx, cacheKey, embedding, data, ttl); indexErr != nil {
		logger.Error("failed to index in cache",
			observability.Error(indexErr),
			observability.String("cache_key", cacheKey))
		return fmt.Errorf("failed to index in cache: %w", indexErr)
	}

	logger.Info("successfully indexed response in cache",
		observability.String("cache_key", cacheKey))
	return nil
}

// Stats returns cache performance metrics.
func (s *SemanticCacheService) Stats(_ context.Context) (*CacheStats, error) {
	return &CacheStats{}, nil
}

// buildQueryText constructs a consistent text representation of the request for embedding.
func (s *SemanticCacheService) buildQueryText(req *CompletionRequest) string {
	var parts []string

	parts = append(parts, fmt.Sprintf("model: %s", req.Model))

	var messages []string
	for _, msg := range req.Messages {
		messages = append(messages, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
	}
	parts = append(parts, fmt.Sprintf("messages: %s", strings.Join(messages, " ")))

	return strings.Join(parts, " | ")
}

// generateCacheKey creates a unique cache key from query text.
func (s *SemanticCacheService) generateCacheKey(queryText string) string {
	hash := sha256.Sum256([]byte(queryText))
	return fmt.Sprintf("cache:%s", hex.EncodeToString(hash[:]))
}
