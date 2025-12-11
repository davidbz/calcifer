# Architecture Issues and Refactoring Guide

This document contains architectural issues identified in the Calcifer AI Gateway codebase, along with detailed instructions for fixing them. Each issue includes context, reasoning, and step-by-step implementation guidance.

---

## 🔴 P0: Critical Issues

### Issue #1: Cost Calculation in Adapter Layer

**Status**: 🔴 Critical
**Files Affected**:
- [internal/provider/openai/adapter.go:244-248](internal/provider/openai/adapter.go#L244-L248)
- [internal/provider/openai/adapter.go:177-190](internal/provider/openai/adapter.go#L177-L190)

**Problem**:
Cost calculation is implemented inside the OpenAI adapter (`toDomainResponse` method), which violates clean architecture principles. The adapter layer should only handle type translation between provider SDK and domain types, not perform business logic.

```go
// Current implementation in adapter.go (WRONG)
modelConfig := p.getModelConfig(string(resp.Model))
inputCost := float64(resp.Usage.PromptTokens) / tokensToPerK * modelConfig.InputCostPer1K
outputCost := float64(resp.Usage.CompletionTokens) / tokensToPerK * modelConfig.OutputCostPer1K
totalCost := inputCost + outputCost
```

**Why It's Wrong**:
1. **Business logic in adapter**: Cost calculation is business logic, not a translation concern
2. **Violates Single Responsibility Principle**: Adapter should only translate types
3. **Untestable in isolation**: Can't test cost calculation without mocking OpenAI SDK
4. **Not reusable**: When adding Anthropic provider, you'll duplicate the logic
5. **Violates "Separate Logic and Data"**: Pricing data is mixed with adapter code

**Solution Architecture**:
```
Current Flow:
  Provider.Complete() → Adapter.toDomainResponse() → [calculates cost] → Response

Desired Flow:
  Provider.Complete() → Adapter.toDomainResponse() → [raw tokens only] → Response
  GatewayService → CostCalculator.Calculate() → [enriches with cost] → Response
```

**Step-by-Step Fix**:

#### Step 1: Create Domain-Level Pricing Package

Create `internal/domain/pricing.go`:

```go
package domain

import (
	"context"
	"errors"
)

// PricingConfig contains model pricing information.
type PricingConfig struct {
	InputCostPer1K  float64 // USD per 1K input tokens
	OutputCostPer1K float64 // USD per 1K output tokens
}

// CostCalculator calculates cost based on token usage.
type CostCalculator interface {
	// Calculate returns the total cost for a given model and usage.
	Calculate(ctx context.Context, model string, usage Usage) (float64, error)
}

// PricingRegistry maintains pricing information for models.
type PricingRegistry interface {
	// GetPricing returns pricing config for a model.
	GetPricing(ctx context.Context, model string) (PricingConfig, error)

	// RegisterPricing adds pricing for a model.
	RegisterPricing(ctx context.Context, model string, config PricingConfig) error
}
```

#### Step 2: Implement Standard Cost Calculator

Create `internal/domain/cost_calculator.go`:

```go
package domain

import (
	"context"
	"fmt"
)

const tokensToPerK = 1000.0

// StandardCostCalculator implements standard token-based cost calculation.
type StandardCostCalculator struct {
	pricingRegistry PricingRegistry
}

// NewStandardCostCalculator creates a new cost calculator.
func NewStandardCostCalculator(registry PricingRegistry) *StandardCostCalculator {
	return &StandardCostCalculator{
		pricingRegistry: registry,
	}
}

// Calculate computes the total cost based on token usage and model pricing.
func (c *StandardCostCalculator) Calculate(
	ctx context.Context,
	model string,
	usage Usage,
) (float64, error) {
	if model == "" {
		return 0, fmt.Errorf("model cannot be empty")
	}

	pricing, err := c.pricingRegistry.GetPricing(ctx, model)
	if err != nil {
		// If pricing not found, return 0 cost (not an error for the request)
		return 0, nil
	}

	inputCost := float64(usage.PromptTokens) / tokensToPerK * pricing.InputCostPer1K
	outputCost := float64(usage.CompletionTokens) / tokensToPerK * pricing.OutputCostPer1K
	totalCost := inputCost + outputCost

	return totalCost, nil
}
```

#### Step 3: Implement In-Memory Pricing Registry

Create `internal/domain/pricing_registry.go`:

```go
package domain

import (
	"context"
	"fmt"
	"sync"
)

// InMemoryPricingRegistry stores pricing configs in memory.
type InMemoryPricingRegistry struct {
	mu      sync.RWMutex
	pricing map[string]PricingConfig
}

// NewInMemoryPricingRegistry creates a new in-memory pricing registry.
func NewInMemoryPricingRegistry() *InMemoryPricingRegistry {
	return &InMemoryPricingRegistry{
		pricing: make(map[string]PricingConfig),
	}
}

// GetPricing retrieves pricing for a model.
func (r *InMemoryPricingRegistry) GetPricing(
	_ context.Context,
	model string,
) (PricingConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	config, exists := r.pricing[model]
	if !exists {
		return PricingConfig{}, fmt.Errorf("pricing not found for model: %s", model)
	}

	return config, nil
}

// RegisterPricing adds pricing for a model.
func (r *InMemoryPricingRegistry) RegisterPricing(
	_ context.Context,
	model string,
	config PricingConfig,
) error {
	if model == "" {
		return fmt.Errorf("model cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.pricing[model] = config
	return nil
}
```

#### Step 4: Create OpenAI Pricing Provider

Create `internal/provider/openai/pricing.go`:

```go
package openai

import (
	"context"

	"github.com/davidbz/calcifer/internal/domain"
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
)

// RegisterPricing registers OpenAI model pricing with the registry.
func RegisterPricing(ctx context.Context, registry domain.PricingRegistry) error {
	models := map[string]domain.PricingConfig{
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

	for model, config := range models {
		if err := registry.RegisterPricing(ctx, model, config); err != nil {
			return err
		}
	}

	return nil
}
```

#### Step 5: Refactor OpenAI Adapter

Modify `internal/provider/openai/adapter.go`:

**Remove**:
- Constants: `gpt4InputCostPer1K`, `gpt4OutputCostPer1K`, etc.
- `tokensToPerK` constant
- `ModelConfig` struct
- `getModelConfig()` method
- Cost calculation logic from `toDomainResponse()`

**Update `toDomainResponse()` method**:
```go
// toDomainResponse converts SDK response to domain response (WITHOUT cost calculation)
func (p *Provider) toDomainResponse(resp *openai.ChatCompletion) *domain.CompletionResponse {
	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	return &domain.CompletionResponse{
		ID:       resp.ID,
		Model:    string(resp.Model),
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
```

#### Step 6: Update GatewayService to Calculate Cost

Modify `internal/domain/gateway.go`:

**Add field to struct**:
```go
type GatewayService struct {
	registry       ProviderRegistry
	costCalculator CostCalculator
}
```

**Update constructor**:
```go
func NewGatewayService(registry ProviderRegistry, costCalculator CostCalculator) *GatewayService {
	return &GatewayService{
		registry:       registry,
		costCalculator: costCalculator,
	}
}
```

**Update `Complete()` method**:
```go
func (g *GatewayService) Complete(
	ctx context.Context,
	providerName string,
	req *CompletionRequest,
) (*CompletionResponse, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	if providerName == "" {
		return nil, errors.New("provider name cannot be empty")
	}

	provider, err := g.registry.Get(ctx, providerName)
	if err != nil {
		return nil, fmt.Errorf("provider not found: %w", err)
	}

	response, err := provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("completion failed: %w", err)
	}

	// Calculate cost in domain layer
	cost, err := g.costCalculator.Calculate(ctx, response.Model, response.Usage)
	if err != nil {
		// Log error but don't fail the request
		// Cost calculation failure shouldn't block the response
	}
	response.Usage.Cost = cost

	return response, nil
}
```

**Update `CompleteByModel()` method similarly**:
```go
func (g *GatewayService) CompleteByModel(
	ctx context.Context,
	req *CompletionRequest,
) (*CompletionResponse, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	if req.Model == "" {
		return nil, errors.New("model cannot be empty")
	}

	provider, err := g.registry.GetByModel(ctx, req.Model)
	if err != nil {
		return nil, fmt.Errorf("provider routing failed: %w", err)
	}

	response, err := provider.Complete(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("completion failed: %w", err)
	}

	// Calculate cost in domain layer
	cost, err := g.costCalculator.Calculate(ctx, response.Model, response.Usage)
	if err != nil {
		// Log error but don't fail the request
	}
	response.Usage.Cost = cost

	return response, nil
}
```

#### Step 7: Update DI Container

Modify `cmd/main.go` (or wherever your DI setup is):

**Add to container**:
```go
// Provide pricing registry
container.Provide(domain.NewInMemoryPricingRegistry)

// Provide cost calculator
container.Provide(func(registry domain.PricingRegistry) domain.CostCalculator {
	return domain.NewStandardCostCalculator(registry)
})

// Update GatewayService constructor call (dig will auto-inject)
container.Provide(domain.NewGatewayService)

// Register OpenAI pricing
container.Invoke(func(registry domain.PricingRegistry) error {
	return openai.RegisterPricing(context.Background(), registry)
})
```

#### Step 8: Write Tests

Create `internal/domain/cost_calculator_test.go`:

```go
package domain_test

import (
	"context"
	"testing"

	"github.com/davidbz/calcifer/internal/domain"
	"github.com/stretchr/testify/require"
)

func TestStandardCostCalculator_Calculate(t *testing.T) {
	ctx := context.Background()
	registry := domain.NewInMemoryPricingRegistry()

	// Register test pricing
	err := registry.RegisterPricing(ctx, "test-model", domain.PricingConfig{
		InputCostPer1K:  0.01,
		OutputCostPer1K: 0.02,
	})
	require.NoError(t, err)

	calculator := domain.NewStandardCostCalculator(registry)

	tests := []struct {
		name           string
		model          string
		usage          domain.Usage
		expectedCost   float64
		expectError    bool
	}{
		{
			name:  "calculate cost for known model",
			model: "test-model",
			usage: domain.Usage{
				PromptTokens:     1000,
				CompletionTokens: 500,
			},
			expectedCost: 0.02, // (1000/1000 * 0.01) + (500/1000 * 0.02)
			expectError:  false,
		},
		{
			name:  "unknown model returns zero cost",
			model: "unknown-model",
			usage: domain.Usage{
				PromptTokens:     1000,
				CompletionTokens: 500,
			},
			expectedCost: 0,
			expectError:  false,
		},
		{
			name:         "empty model returns error",
			model:        "",
			usage:        domain.Usage{},
			expectedCost: 0,
			expectError:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cost, err := calculator.Calculate(ctx, tt.model, tt.usage)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.InDelta(t, tt.expectedCost, cost, 0.0001)
		})
	}
}
```

**Verification Checklist**:
- [ ] All cost calculation logic removed from `adapter.go`
- [ ] Domain layer has `CostCalculator` interface
- [ ] `GatewayService` enriches responses with cost
- [ ] OpenAI pricing registered via DI
- [ ] Tests pass: `go test ./internal/domain/...`
- [ ] Tests pass: `go test ./internal/provider/openai/...`
- [ ] Integration tests still work

---

## 🟡 P1: High Priority Issues

### Issue #2: Model Support Detection via Pricing

**Status**: 🟡 High Priority
**Files Affected**:
- [internal/provider/openai/adapter.go:170-173](internal/provider/openai/adapter.go#L170-L173)

**Problem**:
The `IsModelSupported()` method uses pricing configuration to determine model support:

```go
func (p *Provider) IsModelSupported(_ context.Context, model string) bool {
	config := p.getModelConfig(model)
	return config.InputCostPer1K > 0 || config.OutputCostPer1K > 0
}
```

**Why It's Wrong**:
1. **Conflated concerns**: Model support and pricing are separate concerns
2. **Fragile logic**: What if you support a free model? Or a preview model with no pricing yet?
3. **Implicit magic**: The check `> 0` is not obvious - relies on zero values for unsupported models
4. **Business logic in adapter**: Model support is a business rule, not an adapter concern

**Solution**:

#### Step 1: Create Explicit Model Registry in Provider

Modify `internal/provider/openai/adapter.go`:

**Add field to Provider**:
```go
type Provider struct {
	client          openai.Client
	name            string
	supportedModels map[string]bool
}
```

**Update constructor**:
```go
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

	// Define supported models explicitly
	supportedModels := map[string]bool{
		"gpt-4":         true,
		"gpt-4-turbo":   true,
		"gpt-3.5-turbo": true,
		// Add more models as needed
	}

	return &Provider{
		client:          openai.NewClient(opts...),
		name:            "openai",
		supportedModels: supportedModels,
	}, nil
}
```

**Update `IsModelSupported()`**:
```go
func (p *Provider) IsModelSupported(_ context.Context, model string) bool {
	return p.supportedModels[model]
}
```

#### Step 2: (Optional) Make Models Configurable

Create `internal/provider/openai/models.go`:

```go
package openai

// SupportedModels returns the list of models supported by OpenAI provider.
func SupportedModels() []string {
	return []string{
		"gpt-4",
		"gpt-4-turbo",
		"gpt-4-turbo-preview",
		"gpt-3.5-turbo",
		"gpt-3.5-turbo-16k",
		// Add more as OpenAI releases them
	}
}

// buildModelSet creates a map for O(1) lookup.
func buildModelSet(models []string) map[string]bool {
	set := make(map[string]bool, len(models))
	for _, model := range models {
		set[model] = true
	}
	return set
}
```

**Update constructor**:
```go
func NewProvider(config Config) (*Provider, error) {
	// ... existing validation ...

	return &Provider{
		client:          openai.NewClient(opts...),
		name:            "openai",
		supportedModels: buildModelSet(SupportedModels()),
	}, nil
}
```

#### Step 3: Update Tests

Add to `internal/provider/openai/adapter_test.go`:

```go
func TestProvider_IsModelSupported(t *testing.T) {
	provider, err := NewProvider(Config{
		APIKey: "test-key",
	})
	require.NoError(t, err)

	tests := []struct {
		name      string
		model     string
		supported bool
	}{
		{
			name:      "gpt-4 is supported",
			model:     "gpt-4",
			supported: true,
		},
		{
			name:      "gpt-3.5-turbo is supported",
			model:     "gpt-3.5-turbo",
			supported: true,
		},
		{
			name:      "claude model not supported",
			model:     "claude-3-opus",
			supported: false,
		},
		{
			name:      "empty model not supported",
			model:     "",
			supported: false,
		},
		{
			name:      "unknown model not supported",
			model:     "fake-model-xyz",
			supported: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := provider.IsModelSupported(context.Background(), tt.model)
			require.Equal(t, tt.supported, result)
		})
	}
}
```

**Verification Checklist**:
- [ ] `supportedModels` field added to Provider
- [ ] `IsModelSupported()` uses explicit map lookup
- [ ] No dependency on pricing for model support
- [ ] Tests pass: `go test ./internal/provider/openai/...`
- [ ] Free or preview models can be added without pricing

---

### Issue #3: Stream Goroutine Leak Risk

**Status**: 🟡 High Priority
**Files Affected**:
- [internal/provider/openai/adapter.go:107-162](internal/provider/openai/adapter.go#L107-L162)

**Problem**:
The streaming implementation has a potential goroutine leak if the consumer stops reading from the channel early:

```go
go func() {
	defer close(domainChunks)
	defer logger.Debug("OpenAI stream completed")

	for stream.Next() {
		chunk := stream.Current()
		// ...
		domainChunks <- domain.StreamChunk{...} // BLOCKS if consumer stopped reading
	}
}()
```

**Why It's Wrong**:
1. **Goroutine leak**: If consumer stops reading, the goroutine blocks forever trying to send
2. **No context cancellation**: The goroutine doesn't check `ctx.Done()`
3. **Resource leak**: OpenAI stream isn't properly closed on cancellation
4. **No timeout**: Stream can hang indefinitely if OpenAI has issues

**Solution**:

#### Step 1: Add Context Monitoring to Stream

Modify `internal/provider/openai/adapter.go`:

**Update `Stream()` method**:
```go
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
```

#### Step 2: Update Handler to Use Context with Timeout

Modify `internal/http/handler.go`:

**Update `handleStreamByModel()`**:
```go
func (h *Handler) handleStreamByModel(
	ctx context.Context,
	w http.ResponseWriter,
	req *domain.CompletionRequest,
) {
	logger := observability.FromContext(ctx)
	logger.Info("stream request started")

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Add timeout to prevent indefinite streaming
	streamCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	chunks, err := h.gateway.StreamByModel(streamCtx, req)
	if err != nil {
		logger.Error("stream failed", observability.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		logger.Error("streaming not supported")
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	// Monitor for client disconnect
	notify := w.(http.CloseNotifier).CloseNotify()

	for {
		select {
		case <-notify:
			// Client disconnected
			logger.Info("client disconnected")
			cancel() // Cancel context to stop provider streaming
			return

		case <-streamCtx.Done():
			// Timeout or cancellation
			logger.Error("stream context cancelled", observability.Error(streamCtx.Err()))
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", streamCtx.Err().Error())
			flusher.Flush()
			return

		case chunk, ok := <-chunks:
			if !ok {
				// Channel closed normally
				logger.Info("stream completed normally")
				return
			}

			if chunk.Error != nil {
				logger.Error("stream chunk error", observability.Error(chunk.Error))
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", chunk.Error.Error())
				flusher.Flush()
				return
			}

			// Send chunk as event
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()

			if chunk.Done {
				logger.Info("stream completed")
				return
			}
		}
	}
}
```

**Note**: `http.CloseNotifier` is deprecated in newer Go versions. For Go 1.16+, use request context:

```go
func (h *Handler) handleStreamByModel(
	ctx context.Context,
	w http.ResponseWriter,
	req *domain.CompletionRequest,
) {
	logger := observability.FromContext(ctx)
	logger.Info("stream request started")

	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	// Add timeout to prevent indefinite streaming
	streamCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	chunks, err := h.gateway.StreamByModel(streamCtx, req)
	if err != nil {
		logger.Error("stream failed", observability.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		logger.Error("streaming not supported")
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	for {
		select {
		case <-streamCtx.Done():
			// Timeout, cancellation, or client disconnect
			logger.Info("stream context done", observability.Error(streamCtx.Err()))
			return

		case chunk, ok := <-chunks:
			if !ok {
				// Channel closed normally
				logger.Info("stream completed normally")
				return
			}

			if chunk.Error != nil {
				logger.Error("stream chunk error", observability.Error(chunk.Error))
				fmt.Fprintf(w, "event: error\ndata: %s\n\n", chunk.Error.Error())
				flusher.Flush()
				return
			}

			// Send chunk as event
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()

			if chunk.Done {
				logger.Info("stream completed")
				return
			}
		}
	}
}
```

#### Step 3: Add Configuration for Stream Timeout

Modify `internal/config/config.go`:

**Add field to ServerConfig**:
```go
type ServerConfig struct {
	Port         int           `env:"SERVER_PORT" envDefault:"8080"`
	ReadTimeout  time.Duration `env:"SERVER_READ_TIMEOUT" envDefault:"30s"`
	WriteTimeout time.Duration `env:"SERVER_WRITE_TIMEOUT" envDefault:"30s"`
	StreamTimeout time.Duration `env:"SERVER_STREAM_TIMEOUT" envDefault:"5m"` // NEW
}
```

**Update handler to use config**:
```go
type Handler struct {
	gateway       *domain.GatewayService
	streamTimeout time.Duration
}

func NewHandler(gateway *domain.GatewayService, serverConfig *config.ServerConfig) *Handler {
	return &Handler{
		gateway:       gateway,
		streamTimeout: serverConfig.StreamTimeout,
	}
}

func (h *Handler) handleStreamByModel(ctx context.Context, w http.ResponseWriter, req *domain.CompletionRequest) {
	// ...
	streamCtx, cancel := context.WithTimeout(ctx, h.streamTimeout)
	defer cancel()
	// ...
}
```

**Verification Checklist**:
- [ ] Stream goroutine respects `ctx.Done()`
- [ ] Buffered channel prevents blocking on first chunk
- [ ] `select` statements handle context cancellation
- [ ] Handler has configurable stream timeout
- [ ] Tests verify cancellation behavior
- [ ] No goroutine leaks: `go test -race ./...`

---

## 🟢 P2: Medium Priority Issues

### Issue #4: Hardcoded Model Pricing

**Status**: 🟢 Medium Priority
**Files Affected**:
- After fixing Issue #1, pricing will be in `internal/provider/openai/pricing.go`

**Problem**:
Model pricing is hardcoded in the codebase. OpenAI can change pricing at any time, requiring code redeployment.

**Why It Should Be Configurable**:
1. **Pricing changes frequently**: OpenAI adjusts pricing without notice
2. **No code deployment needed**: Update config file or environment variable
3. **Testing flexibility**: Easier to test with mock pricing
4. **Multi-environment support**: Dev/staging can use different pricing

**Solution**:

#### Step 1: Add Pricing to Configuration

Create `config/pricing.yaml`:

```yaml
# OpenAI Model Pricing (USD per 1K tokens)
openai:
  gpt-4:
    input_cost_per_1k: 0.03
    output_cost_per_1k: 0.06
  gpt-4-turbo:
    input_cost_per_1k: 0.01
    output_cost_per_1k: 0.03
  gpt-3.5-turbo:
    input_cost_per_1k: 0.0005
    output_cost_per_1k: 0.0015

# Anthropic Model Pricing (future)
anthropic:
  claude-3-opus:
    input_cost_per_1k: 0.015
    output_cost_per_1k: 0.075
```

#### Step 2: Create Pricing Config Loader

Create `internal/config/pricing.go`:

```go
package config

import (
	"context"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"

	"github.com/davidbz/calcifer/internal/domain"
)

// PricingConfig represents the pricing configuration file structure.
type PricingConfig struct {
	OpenAI    map[string]ModelPricing `yaml:"openai"`
	Anthropic map[string]ModelPricing `yaml:"anthropic"`
}

// ModelPricing represents pricing for a single model.
type ModelPricing struct {
	InputCostPer1K  float64 `yaml:"input_cost_per_1k"`
	OutputCostPer1K float64 `yaml:"output_cost_per_1k"`
}

// LoadPricing loads pricing configuration from a YAML file.
func LoadPricing(filePath string) (*PricingConfig, error) {
	if filePath == "" {
		filePath = "config/pricing.yaml"
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read pricing config: %w", err)
	}

	var config PricingConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse pricing config: %w", err)
	}

	return &config, nil
}

// RegisterPricingFromConfig registers pricing from config into the registry.
func RegisterPricingFromConfig(
	ctx context.Context,
	registry domain.PricingRegistry,
	config *PricingConfig,
) error {
	// Register OpenAI pricing
	for model, pricing := range config.OpenAI {
		err := registry.RegisterPricing(ctx, model, domain.PricingConfig{
			InputCostPer1K:  pricing.InputCostPer1K,
			OutputCostPer1K: pricing.OutputCostPer1K,
		})
		if err != nil {
			return fmt.Errorf("failed to register pricing for %s: %w", model, err)
		}
	}

	// Register Anthropic pricing
	for model, pricing := range config.Anthropic {
		err := registry.RegisterPricing(ctx, model, domain.PricingConfig{
			InputCostPer1K:  pricing.InputCostPer1K,
			OutputCostPer1K: pricing.OutputCostPer1K,
		})
		if err != nil {
			return fmt.Errorf("failed to register pricing for %s: %w", model, err)
		}
	}

	return nil
}
```

#### Step 3: Update DI Container

Modify `cmd/main.go`:

```go
// Load pricing configuration
container.Provide(func() (*config.PricingConfig, error) {
	pricingPath := os.Getenv("PRICING_CONFIG_PATH")
	if pricingPath == "" {
		pricingPath = "config/pricing.yaml"
	}
	return config.LoadPricing(pricingPath)
})

// Register pricing from config
container.Invoke(func(
	registry domain.PricingRegistry,
	pricingConfig *config.PricingConfig,
) error {
	return config.RegisterPricingFromConfig(context.Background(), registry, pricingConfig)
})
```

#### Step 4: Add Fallback to Hardcoded Pricing

Update `internal/provider/openai/pricing.go` to keep as fallback:

```go
// RegisterPricing registers OpenAI model pricing with the registry.
// This serves as a fallback if pricing config file is not available.
func RegisterPricing(ctx context.Context, registry domain.PricingRegistry) error {
	models := map[string]domain.PricingConfig{
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

	for model, config := range models {
		if err := registry.RegisterPricing(ctx, model, config); err != nil {
			return err
		}
	}

	return nil
}
```

**Update main.go to try config first, fall back to hardcoded**:
```go
container.Invoke(func(
	registry domain.PricingRegistry,
	pricingConfig *config.PricingConfig,
) error {
	// Try to load from config
	err := config.RegisterPricingFromConfig(context.Background(), registry, pricingConfig)
	if err != nil {
		// Fall back to hardcoded pricing
		log.Warn("failed to load pricing config, using hardcoded values", "error", err)
		return openai.RegisterPricing(context.Background(), registry)
	}
	return nil
})
```

#### Step 5: Add go.mod Dependency

```bash
go get gopkg.in/yaml.v3
```

#### Step 6: Update README

Add to `README.md`:

```markdown
#### Pricing Configuration

Pricing is loaded from `config/pricing.yaml` by default. You can override this with:

```bash
export PRICING_CONFIG_PATH=/path/to/custom/pricing.yaml
```

If the pricing config file is not found, the gateway falls back to hardcoded pricing values.

**Format**:
```yaml
openai:
  gpt-4:
    input_cost_per_1k: 0.03
    output_cost_per_1k: 0.06
```
```

**Verification Checklist**:
- [ ] `config/pricing.yaml` created
- [ ] Pricing loader implemented
- [ ] DI container loads pricing from file
- [ ] Fallback to hardcoded pricing works
- [ ] Environment variable `PRICING_CONFIG_PATH` works
- [ ] README updated with pricing config instructions
- [ ] Tests pass: `go test ./...`

---

### Issue #5: Registry Linear Search Performance

**Status**: 🟢 Medium Priority
**Files Affected**:
- [internal/provider/registry/registry.go](internal/provider/registry/registry.go)

**Problem**:
The `GetByModel()` method likely performs a linear search through all providers, calling `IsModelSupported()` on each until it finds a match. This is O(n) complexity.

```go
// Likely current implementation
func (r *Registry) GetByModel(ctx context.Context, model string) (Provider, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, provider := range r.providers {
		if provider.IsModelSupported(ctx, model) {
			return provider, nil
		}
	}

	return nil, errors.New("no provider supports this model")
}
```

**Why It's a Problem**:
1. **O(n) lookup**: Gets slower as you add more providers
2. **Repeated calls**: Every request does the same search
3. **Wasted CPU**: Calling `IsModelSupported()` repeatedly for same models

**Solution**:

#### Step 1: Read Current Registry Implementation

First, verify the current implementation:

```bash
cat internal/provider/registry/registry.go
```

#### Step 2: Add Reverse Index to Registry

Modify `internal/provider/registry/registry.go`:

**Update Registry struct**:
```go
type Registry struct {
	mu              sync.RWMutex
	providers       map[string]domain.Provider
	modelToProvider map[string]string // NEW: model -> provider name mapping
}
```

**Update constructor**:
```go
func NewRegistry() *Registry {
	return &Registry{
		providers:       make(map[string]domain.Provider),
		modelToProvider: make(map[string]string), // NEW
	}
}
```

**Update `Register()` method to build index**:
```go
func (r *Registry) Register(ctx context.Context, provider domain.Provider) error {
	if provider == nil {
		return errors.New("provider cannot be nil")
	}

	name := provider.Name()
	if name == "" {
		return errors.New("provider name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Check if already registered
	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider already registered: %s", name)
	}

	// Register provider
	r.providers[name] = provider

	// Build reverse index: check which models this provider supports
	// You need a list of known models to check against
	// Option A: Check against a predefined list
	knownModels := r.getKnownModels()
	for _, model := range knownModels {
		if provider.IsModelSupported(ctx, model) {
			r.modelToProvider[model] = name
		}
	}

	return nil
}

// getKnownModels returns a list of all known models across providers.
// This can be loaded from config or hardcoded for now.
func (r *Registry) getKnownModels() []string {
	return []string{
		// OpenAI
		"gpt-4",
		"gpt-4-turbo",
		"gpt-4-turbo-preview",
		"gpt-3.5-turbo",
		"gpt-3.5-turbo-16k",
		// Anthropic (future)
		"claude-3-opus",
		"claude-3-sonnet",
		"claude-3-haiku",
		// Add more as needed
	}
}
```

**Update `GetByModel()` to use index**:
```go
func (r *Registry) GetByModel(ctx context.Context, model string) (domain.Provider, error) {
	if model == "" {
		return nil, errors.New("model cannot be empty")
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	// Use reverse index for O(1) lookup
	providerName, exists := r.modelToProvider[model]
	if !exists {
		// Fallback to linear search for unknown models
		// This handles dynamic models not in the known list
		for _, provider := range r.providers {
			if provider.IsModelSupported(ctx, model) {
				return provider, nil
			}
		}
		return nil, fmt.Errorf("no provider found for model: %s", model)
	}

	provider, exists := r.providers[providerName]
	if !exists {
		// This shouldn't happen, but handle gracefully
		return nil, fmt.Errorf("provider not found: %s", providerName)
	}

	return provider, nil
}
```

#### Step 3: (Better Approach) Let Providers Report Their Models

**Add method to Provider interface**:

Modify `internal/domain/interfaces.go`:

```go
type Provider interface {
	// Complete sends a completion request and returns the full response.
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)

	// Stream sends a completion request and returns a stream of chunks.
	Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error)

	// Name returns the provider identifier.
	Name() string

	// IsModelSupported checks if the provider supports the given model.
	IsModelSupported(ctx context.Context, model string) bool

	// SupportedModels returns a list of all models this provider supports. (NEW)
	SupportedModels(ctx context.Context) []string
}
```

**Implement in OpenAI provider**:

Modify `internal/provider/openai/adapter.go`:

```go
func (p *Provider) SupportedModels(_ context.Context) []string {
	models := make([]string, 0, len(p.supportedModels))
	for model := range p.supportedModels {
		models = append(models, model)
	}
	return models
}
```

**Update Registry.Register()**:
```go
func (r *Registry) Register(ctx context.Context, provider domain.Provider) error {
	if provider == nil {
		return errors.New("provider cannot be nil")
	}

	name := provider.Name()
	if name == "" {
		return errors.New("provider name cannot be empty")
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.providers[name]; exists {
		return fmt.Errorf("provider already registered: %s", name)
	}

	r.providers[name] = provider

	// Build reverse index from provider's supported models
	supportedModels := provider.SupportedModels(ctx)
	for _, model := range supportedModels {
		// Handle conflicts: last provider wins for now
		// TODO: Add conflict detection/resolution
		r.modelToProvider[model] = name
	}

	return nil
}
```

#### Step 4: Update Tests

Add to `internal/provider/registry/registry_test.go`:

```go
func TestRegistry_GetByModel_Performance(t *testing.T) {
	ctx := context.Background()
	registry := NewRegistry()

	// Register multiple providers
	for i := 0; i < 10; i++ {
		provider := &mockProvider{
			name: fmt.Sprintf("provider-%d", i),
			models: map[string]bool{
				fmt.Sprintf("model-%d", i): true,
			},
		}
		err := registry.Register(ctx, provider)
		require.NoError(t, err)
	}

	// Lookup should be fast (O(1) with index)
	start := time.Now()
	for i := 0; i < 1000; i++ {
		_, err := registry.GetByModel(ctx, "model-5")
		require.NoError(t, err)
	}
	elapsed := time.Since(start)

	// Should complete 1000 lookups in < 10ms with index
	// (Would take much longer with linear search)
	require.Less(t, elapsed, 10*time.Millisecond)
}
```

**Verification Checklist**:
- [ ] `SupportedModels()` method added to Provider interface
- [ ] OpenAI provider implements `SupportedModels()`
- [ ] Registry builds reverse index on `Register()`
- [ ] `GetByModel()` uses O(1) map lookup
- [ ] Fallback to linear search for unknown models
- [ ] Tests verify performance improvement
- [ ] Tests pass: `go test ./...`

---

## 🟢 P3: Low Priority / Future Improvements

### Issue #6: Missing Input Validation in Domain Models

**Status**: 🟢 Low Priority
**Files Affected**:
- [internal/domain/models.go](internal/domain/models.go)

**Problem**:
Domain models (`CompletionRequest`) don't have validation logic. Invalid values like negative temperature or excessive max_tokens can slip through.

**Solution** (for future milestone):

Create `internal/domain/validation.go`:

```go
package domain

import (
	"fmt"
)

// Validate checks if the completion request is valid.
func (r *CompletionRequest) Validate() error {
	if r.Model == "" {
		return fmt.Errorf("model is required")
	}

	if len(r.Messages) == 0 {
		return fmt.Errorf("at least one message is required")
	}

	for i, msg := range r.Messages {
		if msg.Role == "" {
			return fmt.Errorf("message[%d]: role is required", i)
		}
		if msg.Content == "" {
			return fmt.Errorf("message[%d]: content is required", i)
		}
		if msg.Role != "user" && msg.Role != "assistant" && msg.Role != "system" {
			return fmt.Errorf("message[%d]: invalid role '%s'", i, msg.Role)
		}
	}

	if r.Temperature < 0 || r.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2, got: %f", r.Temperature)
	}

	if r.MaxTokens < 0 {
		return fmt.Errorf("max_tokens cannot be negative, got: %d", r.MaxTokens)
	}

	if r.MaxTokens > 100000 {
		return fmt.Errorf("max_tokens exceeds reasonable limit, got: %d", r.MaxTokens)
	}

	return nil
}
```

Call in gateway service:
```go
func (g *GatewayService) CompleteByModel(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	if req == nil {
		return nil, errors.New("request cannot be nil")
	}

	// Validate request
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("invalid request: %w", err)
	}

	// ... rest of implementation
}
```

---

### Issue #7: No Circuit Breaker for Provider Failures

**Status**: 🟢 Low Priority (Future)
**Implementation**: Use `sony/gobreaker` library

**When to implement**: After you have multiple providers and observe cascading failures.

---

### Issue #8: Domain Models Have JSON Tags

**Status**: 🟢 Philosophical (Acceptable for naive milestone)
**Files Affected**: [internal/domain/models.go](internal/domain/models.go)

**Problem**:
Domain models are tagged with `json:` tags, coupling them to HTTP transport.

**Purist Solution** (only if adding more transports like gRPC):

Create separate DTOs in HTTP layer:

```go
// internal/http/dto/completion.go
package dto

type CompletionRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
	Stream      bool      `json:"stream,omitempty"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ToDomai converts DTO to domain model
func (r *CompletionRequest) ToDomain() *domain.CompletionRequest {
	messages := make([]domain.Message, len(r.Messages))
	for i, m := range r.Messages {
		messages[i] = domain.Message{
			Role:    m.Role,
			Content: m.Content,
		}
	}

	return &domain.CompletionRequest{
		Model:       r.Model,
		Messages:    messages,
		Temperature: r.Temperature,
		MaxTokens:   r.MaxTokens,
		Stream:      r.Stream,
	}
}
```

**Pragmatic Approach** (recommended for now):
- Keep JSON tags in domain models for naive milestone
- Only separate if you add gRPC, GraphQL, or other transports
- Don't over-engineer early

---

## Summary

### Priority Order for Implementation

1. **🔴 P0: Issue #1** - Move cost calculation to domain layer (Critical architectural violation)
2. **🟡 P1: Issue #2** - Separate model support from pricing (High priority)
3. **🟡 P1: Issue #3** - Fix stream goroutine leak risk (High priority)
4. **🟢 P2: Issue #4** - Make pricing configurable (Medium priority, nice to have)
5. **🟢 P2: Issue #5** - Optimize registry lookup (Medium priority, performance)
6. **🟢 P3: Issue #6-8** - Future improvements (Low priority)

### Testing Strategy After Each Fix

After each issue fix, run:

```bash
# Unit tests
go test ./...

# Race detection
go test -race ./...

# Coverage
go test -cover ./...

# Specific package
go test ./internal/domain/...
go test ./internal/provider/openai/...
```

### Verification Commands

```bash
# Check for goroutine leaks
go test -race ./internal/provider/openai -run TestStream

# Check imports
go mod tidy

# Lint
golangci-lint run

# Build
go build -o bin/gateway ./cmd/gateway
```

---

## Additional Resources

- [Clean Architecture](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Go Concurrency Patterns](https://go.dev/blog/pipelines)
- [uber-go/dig Documentation](https://pkg.go.dev/go.uber.org/dig)
- [Context Best Practices](https://go.dev/blog/context)

---

**Document Version**: 1.0
**Last Updated**: 2025-12-11
**Status**: Ready for implementation
