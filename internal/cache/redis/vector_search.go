package redis

import (
	"context"
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/davidbz/calcifer/internal/domain"
	"github.com/davidbz/calcifer/internal/observability"
)

const (
	redisDialectVersion = 2
)

// VectorSearch implements vector similarity search using Redis.
type VectorSearch struct {
	client             *redis.Client
	indexName          string
	embeddingDimension int
}

// NewVectorSearch creates a new Redis vector search adapter.
func NewVectorSearch(client *redis.Client, indexName string, embeddingDimension int) (*VectorSearch, error) {
	v := &VectorSearch{
		client:             client,
		indexName:          indexName,
		embeddingDimension: embeddingDimension,
	}

	if err := v.createIndex(); err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}

	return v, nil
}

// floatsToBytes converts float64 slice to binary byte representation.
func floatsToBytes(fs []float64) []byte {
	const bytesPerFloat32 = 4
	buf := make([]byte, len(fs)*bytesPerFloat32)

	for i, f := range fs {
		// Convert float64 to float32 for Redis compatibility
		f32 := float32(f)
		u := math.Float32bits(f32)
		binary.LittleEndian.PutUint32(buf[i*bytesPerFloat32:], u)
	}

	return buf
}

// Search finds similar vectors above the threshold.
func (v *VectorSearch) Search(
	ctx context.Context,
	embed []float64,
	threshold float64,
	limit int,
) ([]*domain.SearchResult, error) {
	embeddingBytes := floatsToBytes(embed)

	logger := observability.FromContext(ctx)
	logger.Info("starting vector search",
		observability.String("index", v.indexName),
		observability.Int("embedding_dim", len(embed)),
		observability.Float64("threshold", threshold),
		observability.Int("limit", limit))

	query := fmt.Sprintf("*=>[KNN %d @embedding $vec AS score]", limit)

	results, err := v.client.FTSearchWithArgs(ctx, v.indexName, query,
		&redis.FTSearchOptions{
			Return: []redis.FTSearchReturn{
				{FieldName: "data"},
				{FieldName: "indexed_at"},
				{FieldName: "score"},
			},
			DialectVersion: redisDialectVersion,
			Params: map[string]any{
				"vec": embeddingBytes,
			},
		},
	).Result()
	if err != nil {
		logger.Error("vector search failed",
			observability.Error(err))
		return nil, fmt.Errorf("search failed: %w", err)
	}

	logger.Info("vector search completed",
		observability.Int("total_docs", results.Total),
		observability.Int("docs_returned", len(results.Docs)))

	return v.parseSearchResults(ctx, results, threshold), nil
}

// Index stores a vector with associated data.
func (v *VectorSearch) Index(
	ctx context.Context,
	key string,
	embedding []float64,
	data []byte,
	ttl time.Duration,
) error {
	logger := observability.FromContext(ctx)
	logger.Debug("starting vector index",
		observability.String("key", key),
		observability.Int("embedding_dim", len(embedding)),
		observability.Int("data_size", len(data)))

	embeddingBytes := floatsToBytes(embedding)

	pipe := v.client.Pipeline()

	pipe.HSet(ctx, key,
		"embedding", embeddingBytes,
		"data", string(data),
		"indexed_at", time.Now().Unix(),
	)

	if ttl > 0 {
		pipe.Expire(ctx, key, ttl)
	}

	if _, execErr := pipe.Exec(ctx); execErr != nil {
		logger.Error("vector index failed",
			observability.Error(execErr))
		return fmt.Errorf("failed to index: %w", execErr)
	}

	logger.Debug("vector index completed successfully")
	return nil
}

// createIndex creates the Redis search index if it doesn't exist.
func (v *VectorSearch) createIndex() error {
	ctx := context.Background()
	logger := observability.FromContext(ctx)

	// Check if index already exists
	_, err := v.client.FTInfo(ctx, v.indexName).Result()
	if err == nil {
		// Index exists
		logger.Info("redis search index already exists, skipping creation",
			observability.String("index_name", v.indexName))
		return nil
	}

	// Index doesn't exist, create it
	logger.Info("creating redis search index",
		observability.String("index_name", v.indexName),
		observability.Int("embedding_dimension", v.embeddingDimension))

	embeddingDimension := v.embeddingDimension

	_, err = v.client.FTCreate(ctx, v.indexName,
		&redis.FTCreateOptions{
			OnHash: true,
			Prefix: []any{"cache:"},
		},
		&redis.FieldSchema{
			FieldName: "embedding",
			FieldType: redis.SearchFieldTypeVector,
			VectorArgs: &redis.FTVectorArgs{
				FlatOptions: &redis.FTFlatOptions{
					Type:           "FLOAT32",
					Dim:            embeddingDimension,
					DistanceMetric: "COSINE",
				},
			},
		},
		&redis.FieldSchema{
			FieldName: "data",
			FieldType: redis.SearchFieldTypeText,
		},
		&redis.FieldSchema{
			FieldName: "indexed_at",
			FieldType: redis.SearchFieldTypeNumeric,
			Sortable:  true,
		},
	).Result()
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	logger.Info("successfully created redis search index",
		observability.String("index_name", v.indexName))

	return nil
}

// parseSearchResults parses Redis FTSearchResult into domain SearchResult structs.
func (v *VectorSearch) parseSearchResults(
	ctx context.Context,
	result redis.FTSearchResult,
	threshold float64,
) []*domain.SearchResult {
	var results []*domain.SearchResult

	for _, doc := range result.Docs {
		searchResult := v.parseSearchResult(ctx, doc, threshold)
		if searchResult != nil {
			results = append(results, searchResult)
		}
	}

	return results
}

// parseSearchResult parses a single Document into a domain SearchResult.
func (v *VectorSearch) parseSearchResult(
	ctx context.Context,
	doc redis.Document,
	threshold float64,
) *domain.SearchResult {
	logger := observability.FromContext(ctx)

	// Extract score from fields (it's returned as "score" field, not doc.Score)
	scoreStr, scoreOk := doc.Fields["score"]
	if !scoreOk {
		return nil
	}

	score, err := strconv.ParseFloat(scoreStr, 64)
	if err != nil {
		return nil
	}

	// Convert distance to similarity (1.0 - distance for cosine)
	similarity := 1.0 - score

	if similarity < threshold {
		return nil
	}

	// Extract data
	dataStr, dataOk := doc.Fields["data"]
	if !dataOk {
		logger.Warn("data field not found in search result",
			observability.String("key", doc.ID))
		return nil
	}
	// Extract indexed_at
	var indexedAt time.Time
	if tsStr, tsOk := doc.Fields["indexed_at"]; tsOk {
		if ts, parseErr := strconv.ParseInt(tsStr, 10, 64); parseErr == nil {
			indexedAt = time.Unix(ts, 0)
		}
	}

	return &domain.SearchResult{
		Key:        doc.ID,
		Similarity: similarity,
		Data:       []byte(dataStr),
		IndexedAt:  indexedAt,
	}
}
