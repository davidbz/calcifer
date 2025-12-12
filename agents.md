# Calcifer - AI Gateway Project Guide

> This document helps AI assistants understand the Calcifer project structure, architecture, and development workflow.

## Table of Contents
1. [Project Overview](#project-overview)
2. [Architecture & Design](#architecture--design)
3. [Project Structure](#project-structure)
4. [Key Components](#key-components)
5. [Request Flow](#request-flow)
6. [Building & Testing](#building--testing)
7. [Adding New Providers](#adding-new-providers)
8. [Coding Conventions](#coding-conventions)

---

## Project Overview

**Calcifer** is a smart reverse proxy/gateway for Large Language Model (LLM) providers with automatic routing, cost tracking, and semantic caching.

### Core Features
- **Automatic Provider Routing**: Routes requests to the appropriate LLM provider based on model name
- **Cost Calculation**: Tracks and calculates token costs in real-time (USD per request)
- **Semantic Caching**: Optional vector-based caching to reduce costs and latency
- **Provider Agnostic**: Zero vendor lock-in with clean domain abstractions
- **Streaming Support**: Full SSE (Server-Sent Events) streaming for real-time responses
- **Testing**: Built-in echo provider for offline development and testing

### Technology Stack
- **Language**: Go 1.22+
- **Dependency Injection**: uber-go/dig
- **Configuration**: godotenv + caarlos0/env
- **Testing**: Go standard library + testify/require + mockery
- **Caching**: Redis with vector search (RedisStack)
- **HTTP**: Standard library net/http

---

## Architecture & Design

### Architectural Principles

Calcifer follows **Clean Architecture** with strict layer separation:

```
┌─────────────────────────────────────────────┐
│          HTTP Layer (handlers)              │
│  - Request parsing                          │
│  - Response encoding                        │
│  - SSE streaming                            │
└──────────────────┬──────────────────────────┘
                   │
┌──────────────────▼──────────────────────────┐
│      Domain Layer (business logic)          │
│  - GatewayService (orchestration)           │
│  - CostCalculator                           │
│  - SemanticCache                            │
│  - PricingRegistry                          │
│  - ProviderRegistry                         │
│  - Interfaces (Provider, etc.)              │
└──────────────────┬──────────────────────────┘
                   │
┌──────────────────▼──────────────────────────┐
│    Provider Layer (external adapters)       │
│  - OpenAI adapter                           │
│  - Echo adapter (testing)                   │
│  - Future: Anthropic, etc.                  │
└─────────────────────────────────────────────┘
```

### Key Design Patterns

1. **Dependency Injection (DI)**: All components are wired via uber-go/dig container
2. **Interface-Based Design**: Domain defines interfaces, providers implement them
3. **Registry Pattern**: Automatic model → provider routing via reverse index
4. **Observer Pattern**: Logging and cost calculation happen post-response
5. **Adapter Pattern**: Provider implementations translate between domain types and SDK types

### Core Principle
**Providers only translate types. Business logic stays in domain layer.**

---

## Project Structure

```
calcifer/
├── cmd/
│   └── main.go                    # Entry point, DI container setup
│
├── internal/
│   ├── domain/                    # Business logic (NO external dependencies)
│   │   ├── gateway.go            # Request orchestration
│   │   ├── cost_calculator.go    # Token cost calculation
│   │   ├── pricing_registry.go   # Model pricing storage
│   │   ├── semantic_cache.go     # Vector-based caching
│   │   ├── interfaces.go         # Core abstractions (Provider, etc.)
│   │   ├── models.go             # Domain types
│   │   └── *_test.go             # Unit tests
│   │
│   ├── provider/                  # LLM provider adapters
│   │   ├── registry/             # Provider registry implementation
│   │   │   ├── registry.go       # Model → Provider routing
│   │   │   └── registry_test.go
│   │   ├── openai/               # OpenAI provider
│   │   │   ├── adapter.go        # Provider interface implementation
│   │   │   ├── config.go         # Configuration
│   │   │   ├── pricing.go        # Model pricing data
│   │   │   └── models.go         # Type conversions
│   │   └── echo/                 # Test/development provider
│   │       ├── adapter.go        # Echoes input, no API calls
│   │       └── pricing.go        # Zero-cost pricing
│   │
│   ├── httpserver/               # HTTP layer
│   │   ├── handler.go            # Request handlers
│   │   ├── server.go             # HTTP server lifecycle
│   │   └── middleware/           # CORS, tracing, etc.
│   │
│   ├── cache/                    # Caching infrastructure
│   │   └── redis/
│   │       └── vector_search.go  # Redis vector search integration
│   │
│   ├── embedding/                # Embedding generation
│   │   └── openai/
│   │       └── generator.go      # OpenAI embeddings API
│   │
│   ├── config/                   # Configuration management
│   │   └── config.go             # Env var parsing + DI setup
│   │
│   ├── observability/            # Logging & tracing
│   │   ├── logger.go             # Structured logging
│   │   └── context.go            # Context utilities
│   │
│   └── mocks/                    # Auto-generated mocks (mockery)
│       └── *.go                  # DO NOT EDIT MANUALLY
│
├── .github/
│   └── instructions/
│       └── golang.instructions.md # Go coding conventions
│
├── Makefile                       # Build, test, run targets
├── .mockery.yaml                  # Mockery configuration
├── go.mod                         # Go dependencies
└── README.md                      # User-facing documentation
```

---

## Key Components

### 1. GatewayService (`internal/domain/gateway.go`)

**Responsibility**: Orchestrates completion requests, manages caching, enriches responses with cost data.

**Key Methods**:
- `CompleteByModel(ctx, req)` - Route by model, check cache, calculate cost
- `StreamByModel(ctx, req)` - Streaming version with cache wrapper
- `Complete(ctx, providerName, req)` - Direct provider targeting
- `Stream(ctx, providerName, req)` - Direct streaming

**Flow**:
```
Request → Cache Check → Provider Routing → Provider Call → Cost Calculation → Response
```

### 2. Provider Interface (`internal/domain/interfaces.go`)

```go
type Provider interface {
    Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)
    Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error)
    Name() string
    IsModelSupported(ctx context.Context, model string) bool
    SupportedModels(ctx context.Context) []string
}
```

**All providers must implement this interface.**

### 3. ProviderRegistry (`internal/provider/registry/registry.go`)

**Responsibility**: Maps models to providers for automatic routing.

**Implementation**:
- Builds reverse index: `modelToProvider["gpt-4"] = openaiProvider`
- O(1) lookup at request time
- Thread-safe with RWMutex

**Key Methods**:
- `Register(ctx, provider)` - Add provider, index its models
- `GetByModel(ctx, model)` - Find provider for model (automatic routing)
- `Get(ctx, providerName)` - Direct provider lookup
- `List(ctx)` - List all providers

### 4. CostCalculator (`internal/domain/cost_calculator.go`)

**Responsibility**: Calculate USD cost from token usage.

**Formula**:
```
cost = (promptTokens / 1000 * inputCostPer1K) + (completionTokens / 1000 * outputCostPer1K)
```

**Data Source**: PricingRegistry (populated by each provider's `RegisterPricing()` function)

### 5. SemanticCache (`internal/domain/semantic_cache.go`)

**Responsibility**: Cache responses using vector similarity search.

**Flow**:
```
Request → Generate Embedding → Vector Search →
  If similar (>threshold): Return cached response
  Else: Call provider → Cache response
```

**Components**:
- `EmbeddingGenerator`: Converts text to vectors (OpenAI embeddings API)
- `SimilaritySearch`: Finds similar vectors (Redis vector search)
- Threshold: Configurable similarity threshold (default 0.85)

**Cache Key**: SHA256 hash of normalized request text

**Cache Metadata**:
When semantic caching is enabled, cache status is returned via HTTP response headers:

**Cache Hit Headers**:
- `X-Calcifer-Cache: HIT` - Indicates response served from cache
- `X-Calcifer-Cache-Similarity: 0.9600` - Vector similarity score (0.00-1.00)
- `X-Calcifer-Cache-Timestamp: 2024-01-15T09:30:00Z` - When response was cached (RFC3339)
- `X-Calcifer-Cache-Age: 3600` - Age of cached entry in seconds

**Cache Miss Headers**:
- `X-Calcifer-Cache: MISS` - Indicates response from provider (not cached)

**Cache Disabled**:
- No cache headers present

**Note**: Cache hits always have `cost: 0.0` in the usage field (cache hits are free).

**Streaming Limitation**: Cache headers are not currently supported for streaming requests (SSE). Cache status for streaming requests is available in server logs only.

### 6. HTTP Handler (`internal/httpserver/handler.go`)

**Responsibility**: HTTP request/response handling, SSE streaming.

**Endpoints**:
- `POST /v1/completions` - Completion endpoint (streaming & non-streaming)
- `GET /health` - Health check

**Request Parsing**:
```json
{
  "model": "gpt-4",
  "messages": [{"role": "user", "content": "Hello"}],
  "stream": false
}
```

**Response Format** (non-streaming):
```json
{
  "id": "resp-123",
  "model": "gpt-4",
  "provider": "openai",
  "content": "Hello! How can I help?",
  "usage": {
    "prompt_tokens": 12,
    "completion_tokens": 25,
    "total_tokens": 37,
    "cost": 0.00126
  },
  "finish_time": "2024-01-15T10:30:00Z"
}
```

**Cache Headers** (when cache enabled):
```http
X-Calcifer-Cache: MISS
```

**Response Format with Cache Hit**:
```json
{
  "id": "resp-123",
  "model": "gpt-4",
  "provider": "openai",
  "content": "Hello! How can I help?",
  "usage": {
    "prompt_tokens": 12,
    "completion_tokens": 25,
    "total_tokens": 37,
    "cost": 0.0
  },
  "finish_time": "2024-01-15T10:30:00Z"
}
```

**Cache Headers** (cache hit):
```http
X-Calcifer-Cache: HIT
X-Calcifer-Cache-Similarity: 0.9600
X-Calcifer-Cache-Timestamp: 2024-01-15T09:30:00Z
X-Calcifer-Cache-Age: 3600
```

---

## Request Flow

### Non-Streaming Request

```
1. HTTP Request arrives at Handler.HandleCompletion()
   ↓
2. Parse JSON body into CompletionRequest
   ↓
3. Inject model into context for logging
   ↓
4. Call GatewayService.CompleteByModel(ctx, req)
   ├─→ If cache enabled: Check SemanticCache.Get()
   │   ├─→ Cache HIT: Return cached response
   │   └─→ Cache MISS: Continue
   ├─→ ProviderRegistry.GetByModel(req.Model)
   │   └─→ Returns provider (e.g., OpenAI)
   ├─→ Provider.Complete(ctx, req)
   │   └─→ Translates domain types → SDK types
   │   └─→ Calls external API (e.g., OpenAI)
   │   └─→ Translates SDK types → domain types
   ├─→ CostCalculator.Calculate(model, usage)
   │   └─→ Looks up pricing from PricingRegistry
   │   └─→ Computes USD cost
   ├─→ If cache enabled: SemanticCache.Set(req, resp)
   └─→ Return CompletionResponse with cost
   ↓
5. Handler encodes response as JSON
   ↓
6. HTTP Response sent to client
```

### Streaming Request

```
1. HTTP Request with "stream": true
   ↓
2. Handler sets SSE headers (Content-Type: text/event-stream)
   ↓
3. Call GatewayService.StreamByModel(ctx, req)
   ├─→ If cache enabled: Check cache
   │   ├─→ Cache HIT: streamFromCache() (chunked playback)
   │   └─→ Cache MISS: Continue
   ├─→ Provider.Stream(ctx, req)
   │   └─→ Returns <-chan StreamChunk
   ├─→ If cache enabled: cacheStreamWrapper()
   │   └─→ Buffers chunks for caching
   └─→ Return chunk channel
   ↓
4. Handler reads from channel, writes SSE events
   ↓
5. Client receives real-time chunks
```

### Dependency Injection Flow (Startup)

```
main()
  → buildContainer()
    → provideConfig() - Load .env, parse into Config structs
    → provideObservability() - Initialize logger
    → provideRegistries() - Create ProviderRegistry, PricingRegistry
    → provideCostCalculator() - Wire CostCalculator with PricingRegistry
    → provideCache() - Optional: Redis, EmbeddingGenerator, SemanticCache
    → provideEcho() - Always available (no API key needed)
    → provideOpenAI() - If OPENAI_API_KEY set
    → registerProviders() - Call registry.Register() for each provider
    → registerPricing() - Populate PricingRegistry with model prices
    → provideDomainServices() - Create GatewayService
    → provideHTTPLayer() - Create Handler, Middleware, Server
  → container.Invoke(server.Start)
  → Wait for shutdown signal
  → container.Invoke(server.Shutdown)
```

---

## Building & Testing

### Prerequisites
- Go 1.22 or later
- Make
- (Optional) golangci-lint for linting
- (Optional) Redis with RedisStack for semantic caching

### Common Commands

```bash
# Install dependencies
make deps

# Build binary
make build
# Output: bin/app

# Run locally
make run
# Starts server on :8080 (configurable via SERVER_PORT)

# Run tests
make test
# Automatically generates mocks before testing

# Run tests with coverage
make test-coverage
# Generates coverage.html

# Generate mocks only
make mocks

# Regenerate mocks (clean + generate)
make mocks-regen

# Format code
make fmt

# Lint code
make lint

# Clean build artifacts
make clean

# Help
make help
```

### Testing Strategy

1. **Unit Tests**: Test domain logic with mocked dependencies
   - Location: `*_test.go` files next to implementation
   - Mocks: Auto-generated in `internal/mocks/` via mockery
   - Style: Use `require` (not `assert`) for fail-fast behavior

2. **Integration Tests**: Test provider adapters with real APIs
   - Use echo provider for offline tests
   - OpenAI tests require `OPENAI_API_KEY`

3. **Mock Generation**:
   ```bash
   # After changing interfaces in domain/interfaces.go
   make mocks-regen
   ```

### Environment Variables

Create a `.env` file (see README.md for full list):

```bash
# Required for OpenAI provider
OPENAI_API_KEY=sk-...

# Optional: Server configuration
SERVER_PORT=8080

# Optional: Semantic caching
CACHE_ENABLED=true
CACHE_SIMILARITY_THRESHOLD=0.85
REDIS_URL=redis://localhost:6379
```

---

## Adding New Providers

To add a new LLM provider (e.g., Anthropic, Cohere):

### Step 1: Create Provider Package

```
internal/provider/anthropic/
├── adapter.go      # Implement domain.Provider interface
├── config.go       # Configuration struct
├── pricing.go      # RegisterPricing() function
└── models.go       # Type conversion helpers
```

### Step 2: Implement Provider Interface

```go
// internal/provider/anthropic/adapter.go
package anthropic

import (
    "context"
    "github.com/davidbz/calcifer/internal/domain"
)

type Provider struct {
    client *AnthropicClient // SDK client
    name   string
}

func NewProvider(cfg Config) (*Provider, error) {
    client := NewAnthropicClient(cfg.APIKey)
    return &Provider{client: client, name: "anthropic"}, nil
}

func (p *Provider) Name() string {
    return p.name
}

func (p *Provider) Complete(ctx context.Context, req *domain.CompletionRequest) (*domain.CompletionResponse, error) {
    // 1. Convert domain.CompletionRequest → Anthropic SDK request
    sdkReq := convertToAnthropicRequest(req)

    // 2. Call Anthropic API
    sdkResp, err := p.client.CreateMessage(ctx, sdkReq)
    if err != nil {
        return nil, err
    }

    // 3. Convert Anthropic SDK response → domain.CompletionResponse
    return convertToDomainResponse(sdkResp), nil
}

func (p *Provider) Stream(ctx context.Context, req *domain.CompletionRequest) (<-chan domain.StreamChunk, error) {
    // Similar to Complete, but return channel
}

func (p *Provider) IsModelSupported(ctx context.Context, model string) bool {
    supportedModels := map[string]bool{
        "claude-3-opus":   true,
        "claude-3-sonnet": true,
    }
    return supportedModels[model]
}

func (p *Provider) SupportedModels(ctx context.Context) []string {
    return []string{"claude-3-opus", "claude-3-sonnet"}
}
```

### Step 3: Register Pricing

```go
// internal/provider/anthropic/pricing.go
package anthropic

import (
    "context"
    "github.com/davidbz/calcifer/internal/domain"
)

func RegisterPricing(ctx context.Context, registry domain.PricingRegistry) error {
    prices := map[string]domain.PricingConfig{
        "claude-3-opus": {
            InputCostPer1K:  0.015,  // $0.015 per 1K input tokens
            OutputCostPer1K: 0.075,  // $0.075 per 1K output tokens
        },
        "claude-3-sonnet": {
            InputCostPer1K:  0.003,
            OutputCostPer1K: 0.015,
        },
    }

    for model, pricing := range prices {
        if err := registry.RegisterPricing(ctx, model, pricing); err != nil {
            return err
        }
    }
    return nil
}
```

### Step 4: Wire in DI Container

```go
// cmd/main.go

func provideAnthropic(container *dig.Container) {
    mustProvide(container, func(cfg *anthropic.Config) (*anthropic.Provider, error) {
        if cfg.APIKey == "" {
            return nil, ErrProviderNotConfigured
        }
        return anthropic.NewProvider(*cfg)
    })
}

func registerProviders(container *dig.Container) {
    err := container.Invoke(func(
        reg domain.ProviderRegistry,
        echoProvider *echo.Provider,
        openaiProvider *openai.Provider,
        anthropicProvider *anthropic.Provider, // Add new provider
    ) error {
        // ... existing registrations ...

        if anthropicProvider != nil {
            if err := reg.Register(ctx, anthropicProvider); err != nil {
                return fmt.Errorf("failed to register Anthropic: %w", err)
            }
        }
        return nil
    })
    // ...
}

func registerPricing(container *dig.Container) {
    mustInvoke(container, func(pricingReg domain.PricingRegistry) error {
        // ... existing pricing ...

        if err := anthropic.RegisterPricing(ctx, pricingReg); err != nil {
            return fmt.Errorf("failed to register Anthropic pricing: %w", err)
        }
        return nil
    })
}

func buildContainer() *dig.Container {
    container := dig.New()

    provideConfig(container)
    provideObservability(container)
    provideRegistries(container)
    provideCostCalculator(container)
    provideCache(container)
    provideEcho(container)
    provideOpenAI(container)
    provideAnthropic(container)  // Add provider setup
    registerProviders(container)
    registerPricing(container)
    provideDomainServices(container)
    provideHTTPLayer(container)

    return container
}
```

### Step 5: Add Configuration

```go
// internal/config/config.go

type Config struct {
    Server   ServerConfig
    CORS     CORSConfig
    OpenAI   openai.Config
    Anthropic anthropic.Config // Add config
    Cache    CacheConfig
    Redis    RedisConfig
}

// Add to ParseDependenciesConfig if needed
```

**Done!** The registry will automatically route requests for `claude-3-opus` to the Anthropic provider.

---

## Coding Conventions

**IMPORTANT**: This project follows strict Go coding conventions defined in [.github/instructions/golang.instructions.md](.github/instructions/golang.instructions.md).

### Key Conventions Summary

1. **Return Early (Circuit Breaker Pattern)**
   - Handle edge cases first with early returns
   - Reduce nesting, improve readability

2. **Avoid `else` & Use Default Values**
   - Minimize `else` statements
   - Use default values and early returns

3. **Avoid Named Return Values**
   - Use explicit returns for clarity
   - Exception: Defer-based cleanup patterns

4. **Separate Logic and Data**
   - Keep business logic in domain layer
   - Data models in separate files

5. **Context as First Argument**
   - All interface methods must include `ctx context.Context` as first parameter
   - Supports timeouts, cancellation, request-scoped values

6. **Avoid Technology Names in Business Flow**
   - Business logic should be technology-agnostic
   - Use interfaces to abstract external dependencies
   - Example: Use `NotificationService` not `SendGridClient`

7. **Dependency Injection with Dig**
   - All dependencies injected via constructor
   - Registered in `cmd/main.go` buildContainer()
   - No global variables or singletons

8. **Testing: Use `require` (Not `assert`)**
   - Fail fast with `require.NoError(t, err)`
   - Use mockery-generated mocks (NEVER manual mocks)
   - Standard library testing, testify/require for assertions

9. **Configuration**
   - Use `godotenv` to load `.env` files
   - Use `caarlos0/env` for parsing
   - Load once at startup, distribute via DI

10. **Mockery for Test Mocks**
    - ALWAYS use mockery-generated mocks
    - Regenerate after interface changes: `make mocks-regen`
    - Configuration in `.mockery.yaml`

**For detailed examples, see**: [.github/instructions/golang.instructions.md](.github/instructions/golang.instructions.md)

---

## Common Development Scenarios

### Scenario 1: Adding a New Model to Existing Provider

1. Update provider's `IsModelSupported()` and `SupportedModels()`
2. Add pricing in provider's `pricing.go`
3. No other changes needed - registry auto-discovers

### Scenario 2: Debugging Request Flow

1. Check logs for trace IDs (injected via middleware)
2. Look for model routing: `ProviderRegistry.GetByModel()`
3. Verify provider selection in logs
4. Check cost calculation logs

### Scenario 3: Implementing a New Feature

1. Define interface in `internal/domain/interfaces.go`
2. Implement in domain layer
3. Write tests with mockery-generated mocks
4. Wire in DI container (`cmd/main.go`)
5. Expose via HTTP handler if needed

### Scenario 4: Performance Optimization

1. Enable semantic caching (set `CACHE_ENABLED=true`)
2. Tune `CACHE_SIMILARITY_THRESHOLD` (0.85 default)
3. Monitor cache hit rate in logs
4. Consider Redis clustering for scale

---

## Additional Resources

- **User Documentation**: [README.md](README.md)
- **Go Coding Conventions**: [.github/instructions/golang.instructions.md](.github/instructions/golang.instructions.md)
- **Dependency Injection**: [uber-go/dig](https://github.com/uber-go/dig)
- **Mockery**: [vektra/mockery](https://github.com/vektra/mockery)

---

## Tips for AI Assistants

1. **Always check interfaces first** (`internal/domain/interfaces.go`)
2. **Follow the layer boundaries** (HTTP → Domain → Provider)
3. **Use early returns** (see coding conventions)
4. **Generate mocks after interface changes** (`make mocks-regen`)
5. **Test domain logic with mocks** (no external dependencies in tests)
6. **Providers translate types ONLY** (no business logic in adapters)
7. **Wire new components in DI container** (`cmd/main.go`)
8. **Reference the coding conventions** for style questions

When modifying code:
- Read existing implementation patterns first
- Maintain consistency with current architecture
- Update tests alongside code changes
- Run `make test` before committing
- Check `make lint` for style issues
