// Package openai provides an adapter for the OpenAI API using the official SDK.
// It implements the domain.Provider interface and handles conversion between
// domain types and SDK types while preserving business logic for cost calculation
// and model support checking.
package openai

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"

	"github.com/davidbz/calcifer/internal/domain"
	"github.com/davidbz/calcifer/internal/observability"
)

const (
	// GPT-4 pricing per 1K tokens
	gpt4InputCostPer1K  = 0.03
	gpt4OutputCostPer1K = 0.06

	// GPT-4 Turbo pricing per 1K tokens
	gpt4TurboInputCostPer1K  = 0.01
	gpt4TurboOutputCostPer1K = 0.03

	// GPT-3.5 Turbo pricing per 1K tokens
	gpt35TurboInputCostPer1K  = 0.0005
	gpt35TurboOutputCostPer1K = 0.0015

	// Token conversion factor (tokens to per-1K)
	tokensToPerK = 1000.0
)

// ModelConfig contains model configuration including pricing.
type ModelConfig struct {
	InputCostPer1K  float64 // USD per 1K input tokens
	OutputCostPer1K float64 // USD per 1K output tokens
}

// Provider implements the domain.Provider interface for OpenAI
type Provider struct {
	client openai.Client
	name   string
}

// NewProvider creates a new OpenAI provider.
func NewProvider(config Config) (*Provider, error) {
	if config.APIKey == "" {
		return nil, errors.New("OpenAI API key is required")
	}

	opts := []option.RequestOption{
		option.WithAPIKey(config.APIKey),
	}

	if config.BaseURL != "" {
		opts = append(opts, option.WithBaseURL(config.BaseURL))
	}

	if config.Timeout > 0 {
		opts = append(opts, option.WithRequestTimeout(time.Duration(config.Timeout)*time.Second))
	}

	if config.MaxRetries > 0 {
		opts = append(opts, option.WithMaxRetries(config.MaxRetries))
	}

	return &Provider{
		client: openai.NewClient(opts...),
		name:   "openai",
	}, nil
}

// Complete sends a completion request and returns the full response.
func (p *Provider) Complete(ctx context.Context, req *domain.CompletionRequest) (*domain.CompletionResponse, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	logger := observability.FromContext(ctx)
	logger.Debug("calling OpenAI API")

	// Convert domain request to SDK parameters
	params := p.toSDKParams(req)

	// Call OpenAI SDK
	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		logger.Error("OpenAI API call failed", observability.Error(err))
		return nil, fmt.Errorf("OpenAI API call failed: %w", err)
	}

	logger.Debug("OpenAI API call succeeded",
		observability.Int("prompt_tokens", int(resp.Usage.PromptTokens)),
		observability.Int("completion_tokens", int(resp.Usage.CompletionTokens)),
	)

	// Convert SDK response to domain response
	return p.toDomainResponse(resp), nil
}

// Stream sends a completion request and returns a stream of chunks.
func (p *Provider) Stream(ctx context.Context, req *domain.CompletionRequest) (<-chan domain.StreamChunk, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	logger := observability.FromContext(ctx)
	logger.Debug("calling OpenAI streaming API")

	// Convert domain request to SDK parameters
	params := p.toSDKParams(req)

	// Call OpenAI SDK streaming
	stream := p.client.Chat.Completions.NewStreaming(ctx, params)

	// Convert SDK stream to domain chunks channel
	domainChunks := make(chan domain.StreamChunk)

	go func() {
		defer close(domainChunks)
		defer logger.Debug("OpenAI stream completed")

		// Iterate over SDK stream
		for stream.Next() {
			chunk := stream.Current()

			// Extract delta content from choices
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta.Content
				done := chunk.Choices[0].FinishReason != ""

				domainChunks <- domain.StreamChunk{
					Delta: delta,
					Done:  done,
					Error: nil,
				}

				if done {
					return
				}
			}
		}

		// Check for stream errors
		if err := stream.Err(); err != nil {
			if !errors.Is(err, io.EOF) {
				domainChunks <- domain.StreamChunk{
					Delta: "",
					Done:  false,
					Error: fmt.Errorf("OpenAI stream error: %w", err),
				}
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
func (p *Provider) IsModelSupported(_ context.Context, model string) bool {
	config := p.getModelConfig(model)
	return config.InputCostPer1K > 0 || config.OutputCostPer1K > 0
}

// getModelConfig returns the model configuration for a given model.
func (p *Provider) getModelConfig(model string) ModelConfig {
	modelConfigs := map[string]ModelConfig{
		"gpt-4": {
			InputCostPer1K:  gpt4InputCostPer1K,
			OutputCostPer1K: gpt4OutputCostPer1K,
		},
		"gpt-4-turbo": {
			InputCostPer1K:  gpt4TurboInputCostPer1K,
			OutputCostPer1K: gpt4TurboOutputCostPer1K,
		},
		"gpt-3.5-turbo": {
			InputCostPer1K:  gpt35TurboInputCostPer1K,
			OutputCostPer1K: gpt35TurboOutputCostPer1K,
		},
	}

	config, exists := modelConfigs[model]
	if !exists {
		return ModelConfig{
			InputCostPer1K:  0,
			OutputCostPer1K: 0,
		}
	}

	return config
}

// toSDKParams converts domain request to SDK ChatCompletionNewParams
func (p *Provider) toSDKParams(req *domain.CompletionRequest) openai.ChatCompletionNewParams {
	// Convert messages
	messages := make([]openai.ChatCompletionMessageParamUnion, len(req.Messages))
	for i, msg := range req.Messages {
		switch msg.Role {
		case "user":
			messages[i] = openai.UserMessage(msg.Content)
		case "assistant":
			messages[i] = openai.AssistantMessage(msg.Content)
		case "system":
			messages[i] = openai.SystemMessage(msg.Content)
		default:
			// Fallback to user message if role is unknown
			messages[i] = openai.UserMessage(msg.Content)
		}
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(req.Model),
		Messages: messages,
	}

	if req.Temperature > 0 {
		params.Temperature = openai.Float(req.Temperature)
	}

	if req.MaxTokens > 0 {
		params.MaxTokens = openai.Int(int64(req.MaxTokens))
	}

	return params
}

// toDomainResponse converts SDK response to domain response
func (p *Provider) toDomainResponse(resp *openai.ChatCompletion) *domain.CompletionResponse {
	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	// Calculate cost based on token usage and model pricing
	modelConfig := p.getModelConfig(string(resp.Model))
	inputCost := float64(resp.Usage.PromptTokens) / tokensToPerK * modelConfig.InputCostPer1K
	outputCost := float64(resp.Usage.CompletionTokens) / tokensToPerK * modelConfig.OutputCostPer1K
	totalCost := inputCost + outputCost

	return &domain.CompletionResponse{
		ID:       resp.ID,
		Model:    string(resp.Model),
		Provider: p.name,
		Content:  content,
		Usage: domain.Usage{
			PromptTokens:     int(resp.Usage.PromptTokens),
			CompletionTokens: int(resp.Usage.CompletionTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
			Cost:             totalCost,
		},
		FinishTime: time.Now(),
	}
}
