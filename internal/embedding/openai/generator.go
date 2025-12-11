package openai

import (
	"context"
	"errors"
	"fmt"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

const (
	// Embedding dimensions for different OpenAI models.
	embeddingDimensionStandard = 1536 // Ada v2 and Small v3
	embeddingDimensionLarge    = 3072 // Large v3
)

// Generator generates embeddings using OpenAI.
type Generator struct {
	client openai.Client
	model  string
}

// NewGenerator creates a new OpenAI embedding generator.
func NewGenerator(config Config) (*Generator, error) {
	if config.APIKey == "" {
		return nil, errors.New("OpenAI API key is required")
	}

	if config.Model == "" {
		config.Model = string(openai.EmbeddingModelTextEmbeddingAda002)
	}

	return &Generator{
		client: openai.NewClient(option.WithAPIKey(config.APIKey)),
		model:  config.Model,
	}, nil
}

// Generate creates a vector embedding from text.
func (g *Generator) Generate(ctx context.Context, text string) ([]float64, error) {
	if text == "" {
		return nil, errors.New("text cannot be empty")
	}

	//nolint:exhaustruct // OpenAI SDK struct has many optional fields
	resp, err := g.client.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{
			OfArrayOfStrings: []string{text},
		},
		Model: openai.EmbeddingModel(g.model),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create embeddings: %w", err)
	}

	if len(resp.Data) == 0 {
		return nil, errors.New("no embeddings returned")
	}

	return resp.Data[0].Embedding, nil
}

// Name returns the generator identifier.
func (g *Generator) Name() string {
	return "openai"
}

// Dimension returns the vector dimension.
func (g *Generator) Dimension() int {
	switch g.model {
	case string(openai.EmbeddingModelTextEmbeddingAda002),
		string(openai.EmbeddingModelTextEmbedding3Small):
		return embeddingDimensionStandard
	case string(openai.EmbeddingModelTextEmbedding3Large):
		return embeddingDimensionLarge
	default:
		return embeddingDimensionStandard
	}
}
