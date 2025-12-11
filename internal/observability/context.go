package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"

	"github.com/google/uuid"
)

type contextKey string

const (
	traceIDBytes = 16 // OpenTelemetry trace ID size in bytes
	spanIDBytes  = 8  // OpenTelemetry span ID size in bytes
)

const (
	// TraceIDKey holds the OpenTelemetry trace ID.
	TraceIDKey contextKey = "trace_id"

	// SpanIDKey holds the OpenTelemetry span ID.
	SpanIDKey contextKey = "span_id"

	// RequestIDKey holds the unique request identifier.
	RequestIDKey contextKey = "request_id"

	// ProviderKey holds the provider name for this request.
	ProviderKey contextKey = "provider"

	// ModelKey holds the model name for this request.
	ModelKey contextKey = "model"
)

// WithTraceID injects trace ID into context.
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, TraceIDKey, traceID)
}

// WithSpanID injects span ID into context.
func WithSpanID(ctx context.Context, spanID string) context.Context {
	return context.WithValue(ctx, SpanIDKey, spanID)
}

// WithRequestID injects request ID into context.
func WithRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, RequestIDKey, requestID)
}

// WithProvider injects provider name into context.
func WithProvider(ctx context.Context, provider string) context.Context {
	return context.WithValue(ctx, ProviderKey, provider)
}

// WithModel injects model name into context.
func WithModel(ctx context.Context, model string) context.Context {
	return context.WithValue(ctx, ModelKey, model)
}

// GetTraceID extracts trace ID from context.
func GetTraceID(ctx context.Context) string {
	if traceID, ok := ctx.Value(TraceIDKey).(string); ok {
		return traceID
	}
	return ""
}

// GetSpanID extracts span ID from context.
func GetSpanID(ctx context.Context) string {
	if spanID, ok := ctx.Value(SpanIDKey).(string); ok {
		return spanID
	}
	return ""
}

// GetRequestID extracts request ID from context.
func GetRequestID(ctx context.Context) string {
	if requestID, ok := ctx.Value(RequestIDKey).(string); ok {
		return requestID
	}
	return ""
}

// GetProvider extracts provider name from context.
func GetProvider(ctx context.Context) string {
	if provider, ok := ctx.Value(ProviderKey).(string); ok {
		return provider
	}
	return ""
}

// GetModel extracts model name from context.
func GetModel(ctx context.Context) string {
	if model, ok := ctx.Value(ModelKey).(string); ok {
		return model
	}
	return ""
}

// GenerateTraceID generates an OpenTelemetry-compatible trace ID (32 hex chars).
func GenerateTraceID() string {
	bytes := make([]byte, traceIDBytes)
	if _, err := rand.Read(bytes); err != nil {
		return uuid.New().String()
	}
	return hex.EncodeToString(bytes)
}

// GenerateSpanID generates an OpenTelemetry-compatible span ID (16 hex chars).
func GenerateSpanID() string {
	bytes := make([]byte, spanIDBytes)
	if _, err := rand.Read(bytes); err != nil {
		return uuid.New().String()[:16]
	}
	return hex.EncodeToString(bytes)
}

// GenerateRequestID generates a unique request identifier (UUID).
func GenerateRequestID() string {
	return uuid.New().String()
}
