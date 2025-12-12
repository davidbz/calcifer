# Calcifer - AI Gateway

> A smart reverse proxy for LLM providers with automatic routing and cost tracking.

## Overview

Calcifer routes requests to multiple LLM providers based on model name and tracks token costs automatically.

**Key Features:**
- 🎯 Automatic provider routing based on model name
- 💰 Real-time cost calculation (USD per request)
- 🔌 Zero vendor lock-in with provider-agnostic design
- 🚀 SSE streaming support for real-time responses
- 🧠 Semantic caching (optional, reduces costs & latency)

---

## Quick Start

```bash
git clone <repository-url>
cd calcifer
go mod download

# Set API key and start
export OPENAI_API_KEY="sk-..."
make run

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

**Semantic Cache (Optional):**
- `CACHE_ENABLED` - Enable semantic caching (default: false)
- `CACHE_SIMILARITY_THRESHOLD` - Similarity threshold 0-1 (default: 0.85)
- `CACHE_TTL` - Cache entry TTL (default: 1h)
- `CACHE_EMBEDDING_MODEL` - OpenAI embedding model (default: text-embedding-ada-002)
- `REDIS_URL` - Redis connection URL (default: redis://localhost:6379)
- `REDIS_PASSWORD` - Redis password (optional)
- `REDIS_DB` - Redis database number (default: 0)

> **Note:** Semantic cache uses vector similarity to cache responses. Requires Redis with vector search support (RedisStack).

---

## Development

### Building & Testing

```bash
# Build the app
make build

# Run all tests
make test

# Run with coverage
make test-coverage

# Run locally
make run
```

For detailed development documentation including:
- Architecture details and design patterns
- Complete project structure
- Adding new providers (step-by-step)
- Request flow diagrams
- Coding conventions

**See [agents.md](agents.md)** - Developer and AI assistant guide.

---

## License

MIT
