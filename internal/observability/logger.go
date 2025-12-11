package observability

import (
	"context"
	"fmt"
	"sync"

	"go.uber.org/zap"
)

const (
	maxLoggerFieldCapacity int = 5 // Maximum number of context fields to add to logger
)

// Global logger instance - shared across the application.
// This is intentional: loggers should not be stored in context.
//
//nolint:gochecknoglobals // Singleton logger is a standard pattern
var (
	globalLogger *zap.Logger
	loggerMu     sync.RWMutex
)

// InitLogger initializes the base logger (called once at startup).
func InitLogger() (*zap.Logger, error) {
	logger, err := zap.NewProduction()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize logger: %w", err)
	}

	loggerMu.Lock()
	globalLogger = logger
	loggerMu.Unlock()

	return logger, nil
}

// getBaseLogger returns the global logger instance.
func getBaseLogger() *zap.Logger {
	loggerMu.RLock()
	logger := globalLogger
	loggerMu.RUnlock()

	if logger == nil {
		// Fallback to production logger if not initialized
		logger, _ = zap.NewProduction()
	}

	return logger
}

// FromContext creates a logger with fields extracted from context.
func FromContext(ctx context.Context) *zap.Logger {
	logger := getBaseLogger()

	fields := make([]zap.Field, 0, maxLoggerFieldCapacity)

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
