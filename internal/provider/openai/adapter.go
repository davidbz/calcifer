package openai

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/davidbz/calcifer/internal/domain"
)

// Provider implements the domain.Provider interface for OpenAI
type Provider struct {
	client *Client
	name   string
}

// NewProvider creates a new OpenAI provider.
func NewProvider(config Config) (*Provider, error) {
	if config.APIKey == "" {
		return nil, fmt.Errorf("OpenAI API key is required")
	}

	client := NewClient(config)

	return &Provider{
		client: client,
		name:   "openai",
	}, nil
}

// Complete sends a completion request and returns the full response.
func (p *Provider) Complete(ctx context.Context, req *domain.CompletionRequest) (*domain.CompletionResponse, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	// Convert domain request to OpenAI request.
	openAIReq := p.toOpenAIRequest(req)

	// Call OpenAI API.
	resp, err := p.client.Complete(ctx, openAIReq)
	if err != nil {
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	// Convert OpenAI response to domain response.
	return p.toDomainResponse(resp), nil
}

// Stream sends a completion request and returns a stream of chunks.
func (p *Provider) Stream(ctx context.Context, req *domain.CompletionRequest) (<-chan domain.StreamChunk, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	// Convert domain request to OpenAI request.
	openAIReq := p.toOpenAIRequest(req)

	// Call OpenAI streaming API.
	clientChunks, err := p.client.Stream(ctx, openAIReq)
	if err != nil {
		return nil, fmt.Errorf("OpenAI stream call failed: %w", err)
	}

	// Convert client chunks to domain chunks.
	domainChunks := make(chan domain.StreamChunk)

	go func() {
		defer close(domainChunks)

		for chunk := range clientChunks {
			domainChunks <- domain.StreamChunk{
				Delta: chunk.Delta,
				Done:  chunk.Done,
				Error: chunk.Error,
			}
		}
	}()

	return domainChunks, nil
}

// Name returns the provider identifier.
func (p *Provider) Name() string {
	return p.name
}

// IsModelSupported checks if the provider supports the given model.
func (p *Provider) IsModelSupported(ctx context.Context, model string) bool {
	supportedModels := map[string]struct{}{
		"gpt-4":         {},
		"gpt-4-turbo":   {},
		"gpt-3.5-turbo": {},
	}
	_, supported := supportedModels[model]
	return supported
}

// toOpenAIRequest converts domain request to OpenAI request
func (p *Provider) toOpenAIRequest(req *domain.CompletionRequest) openAIRequest {
	messages := make([]openAIMessage, len(req.Messages))
	for i, msg := range req.Messages {
		messages[i] = openAIMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	return openAIRequest{
		Model:       req.Model,
		Messages:    messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		Stream:      req.Stream,
	}
}

// toDomainResponse converts OpenAI response to domain response
func (p *Provider) toDomainResponse(resp *openAIResponse) *domain.CompletionResponse {
	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	return &domain.CompletionResponse{
		ID:       resp.ID,
		Model:    resp.Model,
		Provider: p.name,
		Content:  content,
		Usage: domain.Usage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
		FinishTime: time.Now(),
	}
}
