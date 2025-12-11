package observability

import (
	"context"

	"go.uber.org/zap"
)

var baseLogger *zap.Logger

// InitLogger initializes the base logger (called once at startup).
func InitLogger() (*zap.Logger, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, err
	}
	baseLogger = logger
	return logger, nil
}

// FromContext creates a logger with fields extracted from context.
func FromContext(ctx context.Context) *zap.Logger {
	if baseLogger == nil {
		baseLogger, _ = zap.NewProduction()
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

	return baseLogger.With(fields...)
}
