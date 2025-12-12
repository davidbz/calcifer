package domain_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/davidbz/calcifer/internal/domain"
	"github.com/davidbz/calcifer/internal/mocks"
)

func TestSemanticCacheService_Get_CacheHit(t *testing.T) {
	ctx := context.Background()
	mockEmbedding := mocks.NewMockEmbeddingGenerator(t)
	mockSearch := mocks.NewMockSimilaritySearch(t)

	req := &domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	embedding := []float64{0.1, 0.2, 0.3}
	mockEmbedding.EXPECT().
		Generate(mock.Anything, "model: gpt-4 | messages: user: Hello").
		Return(embedding, nil)

	searchResult := &domain.SearchResult{
		Key:        "cache:abc123",
		Similarity: 0.95,
		Data: []byte(
			`{"id":"cached-123","model":"gpt-4","provider":"openai","content":"Cached response","usage":{"prompt_tokens":0,"completion_tokens":0,"total_tokens":0},"finish_time":"0001-01-01T00:00:00Z"}`,
		),
		IndexedAt: time.Now(),
	}

	mockSearch.EXPECT().
		Search(mock.Anything, embedding, 0.85, 1).
		Return([]*domain.SearchResult{searchResult}, nil)

	service := domain.NewSemanticCacheService(mockEmbedding, mockSearch, 0.85)

	result, err := service.Get(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, "cached-123", result.Response.ID)
	require.InEpsilon(t, 0.95, result.SimilarityScore, 0.001)
}

func TestSemanticCacheService_Get_CacheMiss(t *testing.T) {
	ctx := context.Background()
	mockEmbedding := mocks.NewMockEmbeddingGenerator(t)
	mockSearch := mocks.NewMockSimilaritySearch(t)

	req := &domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	embedding := []float64{0.1, 0.2, 0.3}
	mockEmbedding.EXPECT().
		Generate(mock.Anything, "model: gpt-4 | messages: user: Hello").
		Return(embedding, nil)

	mockSearch.EXPECT().
		Search(mock.Anything, embedding, 0.85, 1).
		Return([]*domain.SearchResult{}, nil)

	service := domain.NewSemanticCacheService(mockEmbedding, mockSearch, 0.85)

	result, err := service.Get(ctx, req)
	require.ErrorIs(t, err, domain.ErrCacheMiss)
	require.Nil(t, result)
}

func TestSemanticCacheService_Get_NilRequest(t *testing.T) {
	ctx := context.Background()
	mockEmbedding := mocks.NewMockEmbeddingGenerator(t)
	mockSearch := mocks.NewMockSimilaritySearch(t)

	service := domain.NewSemanticCacheService(mockEmbedding, mockSearch, 0.85)

	result, err := service.Get(ctx, nil)
	require.Error(t, err)
	require.Nil(t, result)
	require.Equal(t, "request cannot be nil", err.Error())
}

func TestSemanticCacheService_Get_EmbeddingError(t *testing.T) {
	ctx := context.Background()
	mockEmbedding := mocks.NewMockEmbeddingGenerator(t)
	mockSearch := mocks.NewMockSimilaritySearch(t)

	req := &domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	mockEmbedding.EXPECT().
		Generate(mock.Anything, "model: gpt-4 | messages: user: Hello").
		Return(nil, errors.New("embedding failed"))

	service := domain.NewSemanticCacheService(mockEmbedding, mockSearch, 0.85)

	result, err := service.Get(ctx, req)
	require.Error(t, err)
	require.Nil(t, result)
	require.Contains(t, err.Error(), "failed to generate embedding")
}

func TestSemanticCacheService_Set_Success(t *testing.T) {
	ctx := context.Background()
	mockEmbedding := mocks.NewMockEmbeddingGenerator(t)
	mockSearch := mocks.NewMockSimilaritySearch(t)

	req := &domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	resp := &domain.CompletionResponse{
		ID:       "resp-123",
		Model:    "gpt-4",
		Provider: "openai",
		Content:  "Hello! How can I help you?",
	}

	embedding := []float64{0.1, 0.2, 0.3}
	mockEmbedding.EXPECT().
		Generate(mock.Anything, "model: gpt-4 | messages: user: Hello").
		Return(embedding, nil)

	mockSearch.EXPECT().
		Index(mock.Anything, mock.MatchedBy(func(key string) bool {
			return len(key) > 6 && key[:6] == "cache:"
		}), embedding, mock.Anything, 1*time.Hour).
		Return(nil)

	service := domain.NewSemanticCacheService(mockEmbedding, mockSearch, 0.85)

	err := service.Set(ctx, req, resp, 1*time.Hour)
	require.NoError(t, err)
}

func TestSemanticCacheService_Set_NilRequest(t *testing.T) {
	ctx := context.Background()
	mockEmbedding := mocks.NewMockEmbeddingGenerator(t)
	mockSearch := mocks.NewMockSimilaritySearch(t)

	resp := &domain.CompletionResponse{
		ID: "resp-123",
	}

	service := domain.NewSemanticCacheService(mockEmbedding, mockSearch, 0.85)

	err := service.Set(ctx, nil, resp, 1*time.Hour)
	require.Error(t, err)
	require.Equal(t, "request cannot be nil", err.Error())
}

func TestSemanticCacheService_Set_NilResponse(t *testing.T) {
	ctx := context.Background()
	mockEmbedding := mocks.NewMockEmbeddingGenerator(t)
	mockSearch := mocks.NewMockSimilaritySearch(t)

	req := &domain.CompletionRequest{
		Model: "gpt-4",
	}

	service := domain.NewSemanticCacheService(mockEmbedding, mockSearch, 0.85)

	err := service.Set(ctx, req, nil, 1*time.Hour)
	require.Error(t, err)
	require.Equal(t, "response cannot be nil", err.Error())
}
