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

// Provider implements the domain.Provider interface for OpenAI
type Provider struct {
	client          openai.Client
	name            string
	supportedModels map[string]bool
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
		client:          openai.NewClient(opts...),
		name:            "openai",
		supportedModels: buildModelSet(SupportedModels()),
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
//
//nolint:gocognit // Complexity required for proper context cancellation handling
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
	// Use buffered channel to prevent blocking on first chunk
	domainChunks := make(chan domain.StreamChunk, 1)

	go func() {
		defer close(domainChunks)
		defer logger.Debug("OpenAI stream completed")

		// Process stream with context cancellation support
		for stream.Next() {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				logger.Debug("stream cancelled by context")
				// Send cancellation error
				select {
				case domainChunks <- domain.StreamChunk{
					Delta: "",
					Done:  false,
					Error: ctx.Err(),
				}:
				default:
					// Channel full or consumer gone, exit silently
				}
				return
			default:
				// Continue processing
			}

			chunk := stream.Current()

			// Extract delta content from choices
			if len(chunk.Choices) > 0 {
				delta := chunk.Choices[0].Delta.Content
				done := chunk.Choices[0].FinishReason != ""

				streamChunk := domain.StreamChunk{
					Delta: delta,
					Done:  done,
					Error: nil,
				}

				// Try to send chunk, but respect context cancellation
				select {
				case domainChunks <- streamChunk:
					// Successfully sent
				case <-ctx.Done():
					logger.Debug("stream cancelled while sending chunk")
					return
				}

				if done {
					return
				}
			}
		}

		// Check for stream errors
		if err := stream.Err(); err != nil {
			if !errors.Is(err, io.EOF) {
				logger.Error("OpenAI stream error", observability.Error(err))

				// Try to send error, but don't block
				select {
				case domainChunks <- domain.StreamChunk{
					Delta: "",
					Done:  false,
					Error: fmt.Errorf("OpenAI stream error: %w", err),
				}:
				case <-ctx.Done():
					// Context cancelled, exit silently
				default:
					// Channel full, exit (consumer likely gone)
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
	return p.supportedModels[model]
}

// SupportedModels returns a list of all models this provider supports.
func (p *Provider) SupportedModels(_ context.Context) []string {
	models := make([]string, 0, len(p.supportedModels))
	for model := range p.supportedModels {
		models = append(models, model)
	}
	return models
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

	//nolint:exhaustruct // OpenAI SDK struct has many optional fields
	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(req.Model), //nolint:unconvert // Type conversion required by SDK
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

// toDomainResponse converts SDK response to domain response (WITHOUT cost calculation)
func (p *Provider) toDomainResponse(resp *openai.ChatCompletion) *domain.CompletionResponse {
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
			PromptTokens:     int(resp.Usage.PromptTokens),
			CompletionTokens: int(resp.Usage.CompletionTokens),
			TotalTokens:      int(resp.Usage.TotalTokens),
			Cost:             0, // Will be calculated by domain layer
		},
		FinishTime: time.Now(),
	}
}
