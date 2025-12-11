package http

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/davidbz/calcifer/internal/domain"
	"github.com/davidbz/calcifer/internal/observability"
	"go.uber.org/zap"
)

// Handler handles HTTP requests.
type Handler struct {
	gateway *domain.GatewayService
}

// NewHandler creates a new HTTP handler (DI constructor).
func NewHandler(gateway *domain.GatewayService) *Handler {
	return &Handler{
		gateway: gateway,
	}
}

// HandleCompletion processes completion requests.
func (h *Handler) HandleCompletion(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Early validation.
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract provider from header.
	provider := r.Header.Get("X-Provider")
	if provider == "" {
		http.Error(w, "provider not specified in X-Provider header", http.StatusBadRequest)
		return
	}

	// Inject provider into context for downstream logging.
	ctx = observability.WithProvider(ctx, provider)

	// Parse request.
	var req domain.CompletionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("invalid request body: %v", err), http.StatusBadRequest)
		return
	}

	// Inject model into context for downstream logging.
	ctx = observability.WithModel(ctx, req.Model)

	logger := observability.FromContext(ctx)
	logger.Info("completion request received",
		zap.String("provider", provider),
		zap.String("model", req.Model),
		zap.Bool("stream", req.Stream),
	)

	// Handle streaming vs non-streaming.
	if req.Stream {
		h.handleStream(ctx, w, provider, &req)
		return
	}

	// Non-streaming response.
	response, err := h.gateway.Complete(ctx, provider, &req)
	if err != nil {
		logger.Error("completion failed", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logger.Info("completion succeeded",
		zap.Int("tokens", response.Usage.TotalTokens),
		zap.Float64("cost", response.Usage.Cost),
	)

	w.Header().Set("Content-Type", "application/json")
	encodeErr := json.NewEncoder(w).Encode(response)
	if encodeErr != nil {
		logger.Error("failed to encode response", zap.Error(encodeErr))
		http.Error(w, fmt.Sprintf("failed to encode response: %v", encodeErr), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) handleStream(
	ctx context.Context,
	w http.ResponseWriter,
	provider string,
	req *domain.CompletionRequest,
) {
	logger := observability.FromContext(ctx)
	logger.Info("stream request started")

	// Set headers for SSE.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	chunks, err := h.gateway.Stream(ctx, provider, req)
	if err != nil {
		logger.Error("stream failed", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		logger.Error("streaming not supported")
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	for chunk := range chunks {
		if chunk.Error != nil {
			logger.Error("stream chunk error", zap.Error(chunk.Error))
			// Send error as event.
			fmt.Fprintf(w, "event: error\ndata: %s\n\n", chunk.Error.Error())
			flusher.Flush()
			return
		}

		// Send chunk as event.
		data, _ := json.Marshal(chunk)
		fmt.Fprintf(w, "data: %s\n\n", string(data))
		flusher.Flush()

		if chunk.Done {
			logger.Info("stream completed")
			break
		}
	}
}

// HandleHealth handles health check requests.
func (h *Handler) HandleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(map[string]string{
		"status": "healthy",
	}); err != nil {
		// Already written status, can't change it, just log.
		return
	}
}
