package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/davidbz/calcifer/internal/domain"
	"github.com/davidbz/calcifer/internal/observability"
)

// VectorSearch implements vector similarity search using Redis.
type VectorSearch struct {
	client    *redis.Client
	indexName string
}

// NewVectorSearch creates a new Redis vector search adapter.
func NewVectorSearch(client *redis.Client, indexName string) (*VectorSearch, error) {
	v := &VectorSearch{
		client:    client,
		indexName: indexName,
	}

	if err := v.createIndex(); err != nil {
		return nil, fmt.Errorf("failed to create index: %w", err)
	}

	return v, nil
}

// Search finds similar vectors above the threshold.
func (v *VectorSearch) Search(
	ctx context.Context,
	embed []float64,
	threshold float64,
	limit int,
) ([]*domain.SearchResult, error) {
	embeddingBytes, err := json.Marshal(embed)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedding: %w", err)
	}

	logger := observability.FromContext(ctx)
	logger.Debug("starting vector search",
		observability.String("index", v.indexName),
		observability.Int("embedding_dim", len(embed)),
		observability.Float64("threshold", threshold),
		observability.Int("limit", limit))

	query := fmt.Sprintf("*=>[KNN %d @embedding $vec AS score]", limit)
	cmd := v.client.Do(ctx,
		"FT.SEARCH", v.indexName,
		query,
		"PARAMS", "2", "vec", string(embeddingBytes),
		"SORTBY", "score",
		"DIALECT", "2",
		"RETURN", "3", "data", "indexed_at", "score",
	)

	result, err := cmd.Result()
	if err != nil {
		logger.Error("vector search failed",
			observability.Error(err))
		return nil, fmt.Errorf("search failed: %w", err)
	}

	logger.Debug("vector search raw result",
		observability.Any("result", result))

	results := v.parseSearchResults(result, threshold)
	logger.Debug("vector search completed",
		observability.Int("results_count", len(results)))
	return results, nil
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

	embeddingBytes, err := json.Marshal(embedding)
	if err != nil {
		return fmt.Errorf("failed to marshal embedding: %w", err)
	}

	pipe := v.client.Pipeline()

	pipe.HSet(ctx, key,
		"embedding", string(embeddingBytes),
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

// Remove deletes an indexed vector.
func (v *VectorSearch) Remove(ctx context.Context, key string) error {
	if err := v.client.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("failed to delete: %w", err)
	}

	return nil
}

// createIndex creates the Redis search index if it doesn't exist.
func (v *VectorSearch) createIndex() error {
	ctx := context.Background()

	// Check if index exists
	result, err := v.client.Do(ctx, "FT._LIST").Result()
	if err != nil {
		return fmt.Errorf("failed to list indexes: %w", err)
	}

	indexes, indexesOk := result.([]any)
	if indexesOk {
		for _, idx := range indexes {
			if idxName, nameOk := idx.(string); nameOk && idxName == v.indexName {
				// Index exists - drop it to recreate with correct schema
				if dropErr := v.client.Do(ctx, "FT.DROPINDEX", v.indexName).Err(); dropErr != nil {
					return fmt.Errorf("failed to drop existing index: %w", dropErr)
				}
				break
			}
		}
	}

	// Create index with correct vector schema
	err = v.client.Do(ctx,
		"FT.CREATE", v.indexName,
		"ON", "HASH",
		"PREFIX", "1", "cache:",
		"SCHEMA",
		"embedding", "VECTOR", "FLAT", "6",
		"TYPE", "FLOAT64",
		"DIM", "1536",
		"DISTANCE_METRIC", "COSINE",
		"data", "TEXT",
		"indexed_at", "NUMERIC", "SORTABLE",
	).Err()
	if err != nil {
		return fmt.Errorf("failed to create index: %w", err)
	}

	return nil
}

// parseSearchResults parses Redis search results into SearchResult structs.
func (v *VectorSearch) parseSearchResults(result any, threshold float64) []*domain.SearchResult {
	arr, arrOk := result.([]any)
	if !arrOk || len(arr) < 1 {
		return nil
	}

	count, countOk := arr[0].(int64)
	if !countOk || count == 0 {
		return nil
	}

	var results []*domain.SearchResult

	for i := 1; i < len(arr); i += 2 {
		if i+1 >= len(arr) {
			break
		}

		key, keyOk := arr[i].(string)
		if !keyOk {
			continue
		}

		fields, fieldsOk := arr[i+1].([]any)
		if !fieldsOk || len(fields) < 6 {
			continue
		}

		searchResult := v.parseSearchResult(key, fields, threshold)
		if searchResult != nil {
			results = append(results, searchResult)
		}
	}

	return results
}

// parseSearchResult parses a single search result from Redis field data.
func (v *VectorSearch) parseSearchResult(key string, fields []any, threshold float64) *domain.SearchResult {
	var data []byte
	var indexedAt time.Time
	var score float64

	for j := 0; j < len(fields); j += 2 {
		if j+1 >= len(fields) {
			break
		}

		fieldName, fieldOk := fields[j].(string)
		if !fieldOk {
			continue
		}

		v.parseField(fieldName, fields[j+1], &data, &indexedAt, &score)
	}

	if score < threshold {
		return nil
	}

	return &domain.SearchResult{
		Key:        key,
		Similarity: score,
		Data:       data,
		IndexedAt:  indexedAt,
	}
}

// parseField parses a single field from Redis search results.
func (v *VectorSearch) parseField(
	fieldName string,
	fieldValue any,
	data *[]byte,
	indexedAt *time.Time,
	score *float64,
) {
	switch fieldName {
	case "data":
		if dataStr, dataOk := fieldValue.(string); dataOk {
			*data = []byte(dataStr)
		}
	case "indexed_at":
		if tsStr, tsOk := fieldValue.(string); tsOk {
			if ts, parseErr := strconv.ParseInt(tsStr, 10, 64); parseErr == nil {
				*indexedAt = time.Unix(ts, 0)
			}
		}
	case "score":
		if scoreStr, scoreOk := fieldValue.(string); scoreOk {
			if s, parseErr := strconv.ParseFloat(scoreStr, 64); parseErr == nil {
				*score = 1.0 - s // Convert distance to similarity
			}
		}
	}
}
