package observability

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

const loggerKey contextKey = "logger"

// InitLogger initializes the base logger (called once at startup).
func InitLogger() (*zap.Logger, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}
	return logger, nil
}

// WithLogger adds a logger to the context.
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, logger)
}

// FromContext creates a logger with fields extracted from context.
func FromContext(ctx context.Context) *zap.Logger {
	logger, ok := ctx.Value(loggerKey).(*zap.Logger)
	if !ok || logger == nil {
		logger, _ = zap.NewProduction()
	}

	fields := make([]zap.Field, 0, 5)

	if traceID := GetTraceID(ctx); traceID != "" {
		fields = append(fields, zap.String("trace_id", traceID))
	}

	if spanID := GetSpanID(ctx); spanID != "" {
		fields = append(fields, zap.String("span_id", spanID))
	}

	if requestID := GetRequestID(ctx); requestID != "" {
		fields = append(fields, zap.String("request_id", requestID))
	}

	if provider := GetProvider(ctx); provider != "" {
		fields = append(fields, zap.String("provider", provider))
	}

	if model := GetModel(ctx); model != "" {
		fields = append(fields, zap.String("model", model))
	}

	return logger.With(fields...)
}
