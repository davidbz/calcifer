package observability

import (
	"context"
	"log/slog"
)

// EventBus implements the EventPublisher interface.
type EventBus struct {
	logger *slog.Logger
}

// NewEventBus creates a new event bus.
func NewEventBus(logger *slog.Logger) *EventBus {
	return &EventBus{
		logger: logger,
	}
}

// Publish publishes an event with the given type and data.
func (e *EventBus) Publish(ctx context.Context, eventType string, data map[string]interface{}) {
	if e.logger == nil {
		return
	}

	// Convert map to slog attributes.
	attrs := make([]interface{}, 0, len(data)*2)
	for k, v := range data {
		attrs = append(attrs, k, v)
	}

	e.logger.InfoContext(ctx, eventType, attrs...)
}
