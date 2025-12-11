# AI Gateway

A microservices-based AI gateway that acts as a reverse proxy for multiple LLM providers. Currently supports OpenAI, with easy extensibility for additional providers.

## Architecture

The AI Gateway follows clean architecture principles with clear separation of concerns:

- **Domain Layer**: Provider-agnostic business logic
- **Provider Layer**: Provider-specific adapters (OpenAI, Anthropic, etc.)
- **HTTP Layer**: REST API endpoints
- **Observability**: Structured logging and event publishing
- **Configuration**: Environment-based configuration management

See [plan.md](plan.md) for detailed architecture documentation.

## Features

✅ Unified API for multiple LLM providers
✅ Streaming support via Server-Sent Events (SSE)
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
│   └── gateway/
│       └── main.go                 # Application entry point
├── internal/
│   ├── domain/                     # Core business logic
│   │   ├── models.go              # Domain models
│   │   ├── interfaces.go          # Core interfaces
│   │   ├── gateway.go             # Gateway service
│   │   └── gateway_test.go        # Unit tests
│   ├── provider/                   # Provider implementations
│   │   ├── registry/
│   │   │   ├── registry.go        # Provider registry
│   │   │   └── registry_test.go   # Unit tests
│   │   └── openai/
│   │       ├── config.go          # OpenAI configuration
│   │       ├── client.go          # OpenAI HTTP client
│   │       └── adapter.go         # OpenAI adapter
│   ├── routing/                    # Request routing
│   │   ├── router.go
│   │   └── router_test.go
│   ├── http/                       # HTTP layer
│   │   ├── handler.go             # Request handlers
│   │   └── server.go              # HTTP server
│   ├── observability/              # Logging and events
│   │   ├── logger.go
│   │   └── eventbus.go
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

```bash
curl -X POST http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -H "X-Provider: openai" \
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
    "total_tokens": 37
  },
  "finish_time": "2024-01-15T10:30:00Z"
}
```

### Streaming Completion

```bash
curl -X POST http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -H "X-Provider: openai" \
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
go test ./internal/routing
```

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

func (p *Provider) SupportedModels(ctx context.Context) ([]string, error) {
    // Implementation
}
```

3. Add configuration:
```go
// internal/config/config.go
type Config struct {
    Server    ServerConfig
    OpenAI    OpenAIConfig
    Anthropic AnthropicConfig  // Add this
}
```

4. Register in DI container:
```go
// cmd/gateway/main.go
container.Provide(func(cfg *config.Config) (*anthropic.Provider, error) {
    if !cfg.Anthropic.Enabled {
        return nil, nil
    }
    return anthropic.NewProvider(cfg.Anthropic)
})
```

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

## Architecture Decisions

### Why Dependency Injection?
Using `uber-go/dig` enables:
- Testability through interface mocking
- Loose coupling between components
- Easy addition of new providers
- Clear dependency graph

### Why Provider Registry?
The registry pattern allows:
- Dynamic provider registration
- Runtime provider selection
- Easy extensibility
- No hardcoded provider lists in business logic

### Why Event Bus?
Publishing events instead of direct logging:
- Decouples observability from business logic
- Enables multiple event subscribers
- Makes testing easier
- Supports metrics, tracing, and logging from a single source

## Troubleshooting

### "Provider not found" error
- Ensure the provider is enabled via environment variables
- Check that the API key is set
- Verify the provider name in the `X-Provider` header matches a registered provider

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
