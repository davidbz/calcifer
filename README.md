# AI Gateway

A microservices-based AI gateway that acts as a reverse proxy for multiple LLM providers.

## Architecture

The AI Gateway follows clean architecture principles with clear separation of concerns:

- **Domain Layer**: Provider-agnostic business logic and gateway orchestration
- **Provider Layer**: Provider-specific adapters and intelligent routing registry
- **HTTP Layer**: REST API endpoints
- **Observability**: Structured logging and event publishing
- **Configuration**: Environment-based configuration management

## Features

✅ Unified API for multiple LLM providers
✅ Automatic provider routing based on model
✅ Streaming support via Server-Sent Events (SSE)
✅ **Token usage tracking and cost calculation**
✅ CORS support with configurable policies
✅ Composable HTTP middleware infrastructure
✅ Provider abstraction (no vendor lock-in)
✅ Dependency injection with uber-go/dig
✅ Structured logging with slog
✅ Comprehensive unit tests
✅ Type-safe throughout
✅ Follows Go best practices from [coding-conv.md](coding-conv.md)

## Project Structure

```
.
├── cmd/
│   └── main.go                     # Application entry point
├── internal/
│   ├── domain/                     # Core business logic
│   │   ├── models.go              # Domain models
│   │   ├── interfaces.go          # Core interfaces
│   │   ├── gateway.go             # Gateway service with auto-routing
│   │   ├── pricing.go             # Pricing interfaces
│   │   ├── cost_calculator.go     # Token-based cost calculation
│   │   ├── pricing_registry.go    # In-memory pricing storage
│   │   ├── gateway_test.go        # Unit tests
│   │   └── cost_calculator_test.go # Cost calculation tests
│   ├── provider/                   # Provider implementations
│   │   ├── registry/
│   │   │   ├── registry.go        # Provider registry with model routing
│   │   │   └── registry_test.go   # Unit tests
│   │   └── openai/
│   │       ├── config.go          # OpenAI configuration
│   │       ├── adapter.go         # OpenAI adapter (type translation only)
│   │       └── pricing.go         # OpenAI model pricing data
│   ├── http/                       # HTTP layer
│   │   ├── handler.go             # Request handlers
│   │   ├── server.go              # HTTP server
│   │   └── middleware/            # HTTP middlewares
│   │       ├── middleware.go      # Core middleware types
│   │       └── cors.go            # CORS middleware
│   ├── observability/              # Logging and events
│   │   ├── logger.go
│   │   ├── trace.go               # Trace middleware
│   │   └── context.go
│   └── config/                     # Configuration
│       └── config.go
└── go.mod
```

## Quick Start

### Prerequisites

- Go 1.24.5 or higher
- OpenAI API key (optional, only if using OpenAI provider)

### Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd calcifer
```

2. Install dependencies:
```bash
go mod download
```

3. Build the application:
```bash
go build -o bin/gateway ./cmd/gateway
```

### Configuration

Configure the gateway using environment variables:

#### Server Configuration
- `SERVER_PORT` - HTTP server port (default: 8080)
- `SERVER_READ_TIMEOUT` - Read timeout in seconds (default: 30)
- `SERVER_WRITE_TIMEOUT` - Write timeout in seconds (default: 30)

#### CORS Configuration
- `CORS_ALLOWED_ORIGINS` - Comma-separated list of allowed origins (default: *)
- `CORS_ALLOWED_METHODS` - Comma-separated list of allowed HTTP methods (default: GET,POST,PUT,DELETE,OPTIONS)
- `CORS_ALLOWED_HEADERS` - Comma-separated list of allowed headers (default: Content-Type,Authorization)
- `CORS_ALLOW_CREDENTIALS` - Allow credentials (cookies, authorization headers) (default: true)
- `CORS_MAX_AGE` - Preflight cache duration in seconds (default: 86400)

#### OpenAI Configuration
- `OPENAI_API_KEY` - OpenAI API key (required for OpenAI provider)
- `OPENAI_BASE_URL` - OpenAI API base URL (default: https://api.openai.com/v1)
- `OPENAI_TIMEOUT` - Request timeout in seconds (default: 60)
- `OPENAI_MAX_RETRIES` - Maximum retry attempts (default: 3)

### Running the Gateway

```bash
# Set your OpenAI API key
export OPENAI_API_KEY="sk-..."

# Run the gateway
./bin/gateway
```

Or run directly:
```bash
OPENAI_API_KEY="sk-..." go run ./cmd/gateway
```

The gateway will start on `http://localhost:8080` by default.

## API Usage

### Non-Streaming Completion

The gateway automatically routes requests to the appropriate provider based on the model name:

```bash
curl -X POST http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Hello, how are you?"}
    ],
    "temperature": 0.7,
    "max_tokens": 100
  }'
```

Response:
```json
{
  "id": "chatcmpl-...",
  "model": "gpt-4",
  "provider": "openai",
  "content": "Hello! I'm doing well, thank you for asking...",
  "usage": {
    "prompt_tokens": 12,
    "completion_tokens": 25,
    "total_tokens": 37,
    "cost": 0.00126
  },
  "finish_time": "2024-01-15T10:30:00Z"
}
```

### Streaming Completion

```bash
curl -X POST http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [
      {"role": "user", "content": "Write a short poem"}
    ],
    "stream": true
  }'
```

Response (Server-Sent Events):
```
data: {"delta":"The","done":false}

data: {"delta":" sky","done":false}

data: {"delta":" is","done":false}

data: {"delta":"","done":true}
```

### Health Check

```bash
curl http://localhost:8080/health
```

Response:
```json
{
  "status": "healthy"
}
```

## Testing

Run all tests:
```bash
go test ./...
```

Run tests with coverage:
```bash
go test -cover ./...
```

Run tests for a specific package:
```bash
go test ./internal/domain
go test ./internal/provider/registry
go test ./internal/config
```

## Mock Generation

This project uses [mockery](https://github.com/vektra/mockery) to automatically generate test mocks from interfaces.

### Generating Mocks

To generate or regenerate mocks after interface changes:

```bash
make mocks
```

Or directly:

```bash
mockery --config .mockery.yaml
```

### Mock Location

All generated mocks are in `internal/mocks/` with the following files:
- `provider.go` - MockProvider
- `provider_registry.go` - MockProviderRegistry
- `cost_calculator.go` - MockCostCalculator
- `pricing_registry.go` - MockPricingRegistry

### Using Mocks in Tests

```go
import (
    "github.com/davidbz/calcifer/internal/mocks"
    "github.com/stretchr/testify/mock"
)

func TestExample(t *testing.T) {
    // Create mock with test context
    mockProvider := mocks.NewMockProvider(t)

    // Set expectations
    mockProvider.EXPECT().Name().Return("test-provider")
    mockProvider.EXPECT().Complete(mock.Anything, mock.AnythingOfType("*domain.CompletionRequest")).
        Return(&domain.CompletionResponse{...}, nil)

    // Use mock in test
    // ... test code ...

    // Verify expectations met
    mockProvider.AssertExpectations(t)
}
```

### When to Regenerate Mocks

Regenerate mocks whenever you:
- Add new methods to interfaces
- Change method signatures
- Add new interfaces to mock
- Update mockery configuration

## Development

### Adding a New Provider

To add support for a new LLM provider (e.g., Anthropic):

1. Create provider package:
```bash
mkdir -p internal/provider/anthropic
```

2. Implement the provider:
```go
// internal/provider/anthropic/adapter.go
package anthropic

import (
    "context"
    "github.com/davidbz/calcifer/internal/domain"
)

type Provider struct {
    client *Client
    name   string
}

func NewProvider(config Config) (*Provider, error) {
    // Implementation
}

func (p *Provider) Complete(ctx context.Context, req *domain.CompletionRequest) (*domain.CompletionResponse, error) {
    // Implementation
}

func (p *Provider) Stream(ctx context.Context, req *domain.CompletionRequest) (<-chan domain.StreamChunk, error) {
    // Implementation
}

func (p *Provider) Name() string {
    return p.name
}

func (p *Provider) IsModelSupported(ctx context.Context, model string) bool {
    // Return true if this provider supports the given model
    supportedModels := map[string]bool{
        "claude-3-opus":   true,
        "claude-3-sonnet": true,
        "claude-3-haiku":  true,
    }
    return supportedModels[model]
}
```

3. Add pricing data:
```go
// internal/provider/anthropic/pricing.go
package anthropic

import (
    "context"
    "github.com/davidbz/calcifer/internal/domain"
)

const (
    claude3OpusInputCostPer1K   = 0.015
    claude3OpusOutputCostPer1K  = 0.075
    claude3SonnetInputCostPer1K = 0.003
    claude3SonnetOutputCostPer1K = 0.015
)

// RegisterPricing registers Anthropic model pricing with the registry.
func RegisterPricing(ctx context.Context, registry domain.PricingRegistry) error {
    models := map[string]domain.PricingConfig{
        "claude-3-opus": {
            InputCostPer1K:  claude3OpusInputCostPer1K,
            OutputCostPer1K: claude3OpusOutputCostPer1K,
        },
        "claude-3-sonnet": {
            InputCostPer1K:  claude3SonnetInputCostPer1K,
            OutputCostPer1K: claude3SonnetOutputCostPer1K,
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

4. Add configuration:
```go
// internal/config/config.go
type Config struct {
    Server    ServerConfig
    OpenAI    OpenAIConfig
    Anthropic AnthropicConfig  // Add this
}
```

5. Register in DI container and registry:
```go
// cmd/main.go

// 1. Provide the provider
container.Provide(func(cfg *config.Config) (*anthropic.Provider, error) {
    if cfg.Anthropic.APIKey == "" {
        return nil, ErrProviderNotConfigured
    }
    return anthropic.NewProvider(cfg.Anthropic)
})

// 2. Register it with the registry (in the Invoke section)
container.Invoke(func(reg domain.ProviderRegistry, anthropicProvider *anthropic.Provider) error {
    if anthropicProvider != nil {
        return reg.Register(ctx, anthropicProvider)
    }
    return nil
})

// 3. Register pricing
container.Invoke(func(pricingReg domain.PricingRegistry) error {
    ctx := context.Background()
    return anthropic.RegisterPricing(ctx, pricingReg)
})
```

The registry will automatically route requests to your provider based on model support, and costs will be calculated automatically using the registered pricing.

### Code Conventions

This project follows strict Go coding conventions documented in [coding-conv.md](coding-conv.md):

- ✅ Early returns (circuit breaker pattern)
- ✅ Avoid `else` statements
- ✅ No named return values
- ✅ Context as first argument in all interfaces
- ✅ Separation of logic and data
- ✅ No technology names in business logic
- ✅ Dependency injection using dig
- ✅ Use `require` (not `assert`) in tests

## Cost Calculation Architecture

The gateway automatically calculates and tracks the cost of LLM requests based on token usage:

### How It Works

1. **Provider Layer**: Adapters return raw token counts (no cost calculation)
2. **Domain Layer**: `GatewayService` enriches responses with cost using `CostCalculator`
3. **Pricing Registry**: In-memory registry stores pricing per model (USD per 1K tokens)
4. **Clean Separation**: Business logic (cost calculation) is separated from type translation (adapters)

### Cost Calculation Flow

```
Request → Provider → Adapter → [tokens only] → Response
                                                    ↓
                                          GatewayService
                                                    ↓
                                          CostCalculator ← PricingRegistry
                                                    ↓
                                          Response + Cost
```

### Benefits

- **Testable**: Cost calculation can be tested independently of provider adapters
- **Reusable**: All providers share the same cost calculation logic
- **Maintainable**: Pricing updates don't require adapter changes
- **Clean Architecture**: Business logic stays in the domain layer

### Supported Models

Current pricing (as of implementation):

| Model | Input (per 1K tokens) | Output (per 1K tokens) |
|-------|----------------------|------------------------|
| gpt-4 | $0.03 | $0.06 |
| gpt-4-turbo | $0.01 | $0.03 |
| gpt-3.5-turbo | $0.0005 | $0.0015 |

## Architecture Decisions

### Why Dependency Injection?
Using `uber-go/dig` enables:
- Testability through interface mocking
- Loose coupling between components
- Easy addition of new providers
- Clear dependency graph

### Why Provider Registry?
The registry pattern with integrated routing allows:
- Dynamic provider registration
- Automatic model-based provider selection via `GetByModel()`
- Type-safe provider discovery (returns Provider interface, not strings)
- Single source of truth for provider management
- Eliminates unnecessary abstraction layers

### Why Event Bus?
Publishing events instead of direct logging:
- Decouples observability from business logic
- Enables multiple event subscribers
- Makes testing easier
- Supports metrics, tracing, and logging from a single source

### Why Middleware Infrastructure?
Composable middleware chain enables:
- Separation of concerns (CORS, auth, tracing as independent components)
- Explicit ordering and composition via `middleware.Chain()`
- Easy addition of future middlewares (auth, rate limiting, metrics)
- Type-safe middleware contracts
- Testable in isolation
- Follows standard Go middleware patterns

## Troubleshooting

### "Provider not found" error
- Ensure the provider is enabled via environment variables
- Check that the API key is set
- Verify the model name is supported by at least one configured provider

### Connection timeouts
- Increase `OPENAI_TIMEOUT` environment variable
- Check network connectivity to the LLM provider
- Verify API key is valid

### Build failures
- Ensure Go 1.24.5+ is installed
- Run `go mod tidy` to sync dependencies
- Check for syntax errors in your code

## License

MIT License - See LICENSE file for details

## Contributing

1. Fork the repository
2. Create a feature branch
3. Follow the coding conventions in [coding-conv.md](coding-conv.md)
4. Write unit tests using `require` assertions
5. Ensure all tests pass: `go test ./...`
6. Submit a pull request

## Future Enhancements

- [ ] Add authentication middleware (JWT/API keys)
- [ ] Add rate limiting middleware
- [ ] Add Anthropic provider
- [ ] Add Google (Gemini) provider
- [ ] Implement request/response caching
- [ ] Add rate limiting
- [ ] Add authentication/authorization
- [ ] Implement request retries with exponential backoff
- [ ] Add metrics endpoint (Prometheus)
- [ ] Add distributed tracing (OpenTelemetry)
- [ ] Implement circuit breaker pattern
- [ ] Add load balancing across multiple provider instances
