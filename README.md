# Calcifer - AI Gateway

> A smart reverse proxy for LLM providers with automatic routing and cost tracking.

## Overview

Calcifer routes requests to multiple LLM providers based on model name and tracks token costs automatically.

**Key Features:**
- 🎯 Automatic provider routing
- 💰 Real-time cost calculation (USD per request)
- 🔌 Zero vendor lock-in (provider-agnostic domain)
- 🚀 SSE streaming support
- 🧪 98.6% test coverage in domain layer

**Architecture:** Clean separation - Domain (business logic) → Provider (adapters) → HTTP (handlers)

---

## Quick Start

```bash
git clone <repository-url>
cd calcifer
go mod download

# Set API key and start
export OPENAI_API_KEY="sk-..."
go run ./cmd/main.go

# Gateway running on http://localhost:8080
```

### Usage Example

```bash
# Request routes automatically based on model
curl -X POST http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-4",
    "messages": [{"role": "user", "content": "Hello"}]
  }'
```

**Response includes cost:**
```json
{
  "content": "Hello! How can I help?",
  "usage": {
    "prompt_tokens": 12,
    "completion_tokens": 25,
    "cost": 0.00126
  }
}
```

### Testing Without API Keys

Use the built-in `echo4` model for testing (no API key required):

```bash
# Echo provider returns your input with token counts
curl -X POST http://localhost:8080/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "echo4",
    "messages": [{"role": "user", "content": "Test message"}]
  }'

# Response:
# {
#   "content": "Test message",
#   "usage": {
#     "prompt_tokens": 2,
#     "completion_tokens": 2,
#     "cost": 0.0002
#   }
# }
```

The `echo4` provider:
- Echoes back your input messages
- Calculates token counts (simple word-based)
- Works offline (no external API calls)
- Perfect for development and testing

---

## Configuration

Environment variables:

**Server:**
- `SERVER_PORT` - Port (default: 8080)
- `SERVER_READ_TIMEOUT` - Read timeout (default: 30s)
- `SERVER_WRITE_TIMEOUT` - Write timeout (default: 30s)

**CORS:**
- `CORS_ALLOWED_ORIGINS` - Allowed origins (default: `*`)
- `CORS_ALLOWED_METHODS` - HTTP methods (default: GET,POST,PUT,DELETE,OPTIONS)
- `CORS_ALLOWED_HEADERS` - Headers (default: Content-Type,Authorization)

**OpenAI:**
- `OPENAI_API_KEY` - API key (required)
- `OPENAI_BASE_URL` - Base URL (default: https://api.openai.com/v1)
- `OPENAI_TIMEOUT` - Timeout (default: 60s)
- `OPENAI_MAX_RETRIES` - Max retries (default: 3)

---

## Adding a New Provider

Add a new LLM provider in 3 steps:

**1. Implement Provider Interface:**
```go
// internal/provider/anthropic/adapter.go
type Provider struct {
    client *Client
    name   string
}

func (p *Provider) Complete(ctx context.Context, req *domain.CompletionRequest) (*domain.CompletionResponse, error) {
    // Convert domain → SDK types, call API, convert back
}

func (p *Provider) Stream(ctx context.Context, req *domain.CompletionRequest) (<-chan domain.StreamChunk, error) {
    // Stream implementation
}

func (p *Provider) IsModelSupported(ctx context.Context, model string) bool {
    return supportedModels[model]
}
```

**2. Register Pricing:**
```go
// internal/provider/anthropic/pricing.go
func RegisterPricing(ctx context.Context, registry domain.PricingRegistry) error {
    return registry.RegisterPricing(ctx, "claude-3-opus", domain.PricingConfig{
        InputCostPer1K:  0.015,  // $0.015 per 1K tokens
        OutputCostPer1K: 0.075,  // $0.075 per 1K tokens
    })
}
```

**3. Wire in DI Container:**
```go
// cmd/main.go
container.Provide(func(cfg *config.Config) (*anthropic.Provider, error) {
    return anthropic.NewProvider(cfg.Anthropic)
})

container.Invoke(func(reg domain.ProviderRegistry, p *anthropic.Provider) error {
    return reg.Register(ctx, p)
})

container.Invoke(func(pricingReg domain.PricingRegistry) error {
    return anthropic.RegisterPricing(ctx, pricingReg)
})
```

**Done.** The registry automatically routes requests based on model name and calculates costs.

---

## Architecture

### Layer Separation

```
HTTP Layer (handlers, middleware)
    ↓
Domain Layer (business logic, interfaces)
    ↓
Provider Layer (adapters for OpenAI, Anthropic, etc.)
```

**Key Principle:** Providers only translate types. Business logic stays in domain.

### How It Works

**Automatic Routing:**
```go
// Registry builds reverse index: model → provider
registry.Register(ctx, openaiProvider)
// Internally: modelToProvider["gpt-4"] = "openai"

// O(1) lookup at request time
provider := registry.GetByModel(ctx, "gpt-4")
```

**Cost Calculation:**
```
Provider returns tokens → GatewayService → CostCalculator → PricingRegistry → Response + Cost
```

Providers don't calculate cost. Domain layer enriches responses.

---

## Project Structure

```
.
├── cmd/main.go                    # Entry point, DI container
├── internal/
│   ├── domain/                    # Business logic
│   │   ├── gateway.go            # Orchestration
│   │   ├── cost_calculator.go    # Cost calculation
│   │   ├── pricing_registry.go   # Pricing storage
│   │   └── interfaces.go         # Core interfaces
│   ├── provider/
│   │   ├── registry/             # Provider registry
│   │   ├── openai/               # OpenAI adapter
│   │   └── echo/                 # Test provider
│   ├── http/
│   │   ├── handler.go            # HTTP handlers
│   │   ├── server.go             # Server
│   │   └── middleware/           # CORS, tracing
│   ├── config/                    # Configuration
│   └── observability/             # Logging
└── go.mod
```

---

## Testing

```bash
# Run all tests
go test ./...

# With coverage
go test -cover ./...

# Regenerate mocks
make mocks
```
