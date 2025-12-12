// Package echo provides a testing provider that echoes back input messages.
// It implements the domain.Provider interface without making external API calls,
// providing deterministic responses for testing and development purposes.
package echo

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/davidbz/calcifer/internal/domain"
	"github.com/davidbz/calcifer/internal/observability"
)

const (
	providerName = "echo"
	modelName    = "echo4"
	chunkDelay   = 10 * time.Millisecond
)

// Provider implements the domain.Provider interface for echo testing.
type Provider struct {
	name            string
	supportedModels map[string]bool
}

// NewProvider creates a new echo provider.
// No configuration is required as this provider operates entirely in-memory.
func NewProvider() *Provider {
	return &Provider{
		name: providerName,
		supportedModels: map[string]bool{
			modelName: true,
		},
	}
}

// Complete sends a completion request and returns the echoed response.
func (p *Provider) Complete(ctx context.Context, req *domain.CompletionRequest) (*domain.CompletionResponse, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	if !p.supportedModels[req.Model] {
		return nil, fmt.Errorf("model %s is not supported by echo provider", req.Model)
	}

	logger := observability.FromContext(ctx)
	logger.Debug("echoing request")

	// Build echo content from messages
	echoContent := buildEchoContent(req.Messages)

	// Count tokens (simple word-based counting)
	promptTokens := countTokens(echoContent)
	completionTokens := promptTokens // Echo returns same size
	totalTokens := promptTokens + completionTokens

	logger.Debug("echo completed",
		observability.Int("prompt_tokens", promptTokens),
		observability.Int("completion_tokens", completionTokens),
	)

	return &domain.CompletionResponse{
		ID:       fmt.Sprintf("echo-%d", time.Now().UnixNano()),
		Model:    req.Model,
		Provider: p.name,
		Content:  echoContent,
		Usage: domain.Usage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      totalTokens,
			Cost:             0.0,
		},
		FinishTime: time.Now(),
	}, nil
}

// Stream sends a completion request and returns a stream of echo chunks.
func (p *Provider) Stream(ctx context.Context, req *domain.CompletionRequest) (<-chan domain.StreamChunk, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	if !p.supportedModels[req.Model] {
		return nil, fmt.Errorf("model %s is not supported by echo provider", req.Model)
	}

	logger := observability.FromContext(ctx)
	logger.Debug("streaming echo request")

	// Build echo content
	echoContent := buildEchoContent(req.Messages)

	// Create output channel
	chunks := make(chan domain.StreamChunk)

	// Stream chunks in a goroutine
	go func() {
		defer close(chunks)

		// Split content into words for streaming
		words := strings.Fields(echoContent)
		if len(words) == 0 {
			// Send empty done chunk
			select {
			case chunks <- domain.StreamChunk{Delta: "", Done: true, Error: nil}:
			case <-ctx.Done():
			}
			return
		}

		// Stream each word with a small delay
		for i, word := range words {
			delta := word
			if i < len(words)-1 {
				delta += " " // Add space between words
			}

			select {
			case <-ctx.Done():
				chunks <- domain.StreamChunk{
					Delta: "",
					Done:  true,
					Error: ctx.Err(),
				}
				return
			case chunks <- domain.StreamChunk{Delta: delta, Done: false, Error: nil}:
				time.Sleep(chunkDelay)
			}
		}

		// Send final done chunk
		select {
		case chunks <- domain.StreamChunk{Delta: "", Done: true, Error: nil}:
		case <-ctx.Done():
		}
	}()

	return chunks, nil
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

// buildEchoContent constructs the echo response from request messages.
func buildEchoContent(messages []domain.Message) string {
	if len(messages) == 0 {
		return ""
	}

	var builder strings.Builder
	for _, msg := range messages {
		builder.WriteString(fmt.Sprintf("[%s]: %s\n", msg.Role, msg.Content))
	}
	return builder.String()
}

// countTokens performs simple word-based token counting.
func countTokens(content string) int {
	if content == "" {
		return 0
	}
	return len(strings.Fields(content))
}
