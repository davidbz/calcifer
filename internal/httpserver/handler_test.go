package httpserver //nolint:testpackage // Need access to unexported setCacheHeaders function

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/davidbz/calcifer/internal/domain"
	"github.com/davidbz/calcifer/internal/mocks"
)

func TestHandleCompletion_CacheHit_SetsHeaders(t *testing.T) {
	mockRegistry := mocks.NewMockProviderRegistry(t)
	mockCostCalc := mocks.NewMockCostCalculator(t)
	mockCache := mocks.NewMockSemanticCache(t)

	gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, mockCache)
	handler := NewHandler(gateway)

	cachedAt := time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)
	cachedResponse := &domain.CompletionResponse{
		ID:       "resp-123",
		Model:    "gpt-4",
		Provider: "openai",
		Content:  "Hello! How can I help?",
		Usage: domain.Usage{
			PromptTokens:     12,
			CompletionTokens: 25,
			TotalTokens:      37,
			Cost:             0.0,
		},
		FinishTime: time.Now(),
	}

	req := domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
	}

	// Mock cache hit
	mockCache.EXPECT().
		Get(mock.Anything, &req).
		Return(&domain.CachedResponse{
			Response:        cachedResponse,
			SimilarityScore: 0.96,
			CachedAt:        cachedAt,
		}, nil)

	reqBody, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler.HandleCompletion(w, httpReq)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "HIT", w.Header().Get("X-Calcifer-Cache"))
	require.Equal(t, "0.9600", w.Header().Get("X-Calcifer-Cache-Similarity"))
	require.Equal(t, cachedAt.Format(time.RFC3339), w.Header().Get("X-Calcifer-Cache-Timestamp"))

	// Age should be > 0 since we're comparing against time.Now()
	ageHeader := w.Header().Get("X-Calcifer-Cache-Age")
	require.NotEmpty(t, ageHeader)

	// Verify JSON response doesn't contain cache field
	var response domain.CompletionResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	require.Equal(t, "resp-123", response.ID)
	require.Equal(t, "gpt-4", response.Model)
	require.InDelta(t, 0.0, response.Usage.Cost, 0.0001)
}

func TestHandleCompletion_CacheMiss_SetsHeaders(t *testing.T) {
	mockRegistry := mocks.NewMockProviderRegistry(t)
	mockCostCalc := mocks.NewMockCostCalculator(t)
	mockCache := mocks.NewMockSemanticCache(t)
	mockProvider := mocks.NewMockProvider(t)

	gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, mockCache)
	handler := NewHandler(gateway)

	providerResponse := &domain.CompletionResponse{
		ID:       "resp-456",
		Model:    "gpt-4",
		Provider: "openai",
		Content:  "Hello! How can I help?",
		Usage: domain.Usage{
			PromptTokens:     12,
			CompletionTokens: 25,
			TotalTokens:      37,
		},
		FinishTime: time.Now(),
	}

	req := domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
	}

	// Mock cache miss
	mockCache.EXPECT().
		Get(mock.Anything, &req).
		Return(nil, domain.ErrCacheMiss)

	// Mock provider routing
	mockRegistry.EXPECT().
		GetByModel(mock.Anything, "gpt-4").
		Return(mockProvider, nil)

	// Mock provider call
	mockProvider.EXPECT().
		Complete(mock.Anything, &req).
		Return(providerResponse, nil)

	// Mock cost calculation
	mockCostCalc.EXPECT().
		Calculate(mock.Anything, "gpt-4", providerResponse.Usage).
		Return(0.00126, nil)

	// Mock cache set (Set takes 4 parameters: ctx, req, resp, ttl)
	mockCache.EXPECT().
		Set(mock.Anything, &req, mock.MatchedBy(func(resp *domain.CompletionResponse) bool {
			return resp.Usage.Cost == 0.00126
		}), mock.Anything).
		Return(nil)

	reqBody, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler.HandleCompletion(w, httpReq)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "MISS", w.Header().Get("X-Calcifer-Cache"))
	require.Empty(t, w.Header().Get("X-Calcifer-Cache-Similarity"))
	require.Empty(t, w.Header().Get("X-Calcifer-Cache-Timestamp"))
	require.Empty(t, w.Header().Get("X-Calcifer-Cache-Age"))

	// Verify JSON response
	var response domain.CompletionResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	require.Equal(t, "resp-456", response.ID)
	require.InDelta(t, 0.00126, response.Usage.Cost, 0.0001)
}

func TestHandleCompletion_CacheDisabled_NoHeaders(t *testing.T) {
	mockRegistry := mocks.NewMockProviderRegistry(t)
	mockCostCalc := mocks.NewMockCostCalculator(t)
	mockProvider := mocks.NewMockProvider(t)

	// Gateway with nil cache (cache disabled)
	gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)
	handler := NewHandler(gateway)

	providerResponse := &domain.CompletionResponse{
		ID:       "resp-789",
		Model:    "gpt-4",
		Provider: "openai",
		Content:  "Hello! How can I help?",
		Usage: domain.Usage{
			PromptTokens:     12,
			CompletionTokens: 25,
			TotalTokens:      37,
		},
		FinishTime: time.Now(),
	}

	req := domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
	}

	// Mock provider routing
	mockRegistry.EXPECT().
		GetByModel(mock.Anything, "gpt-4").
		Return(mockProvider, nil)

	// Mock provider call
	mockProvider.EXPECT().
		Complete(mock.Anything, &req).
		Return(providerResponse, nil)

	// Mock cost calculation
	mockCostCalc.EXPECT().
		Calculate(mock.Anything, "gpt-4", providerResponse.Usage).
		Return(0.00126, nil)

	reqBody, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler.HandleCompletion(w, httpReq)

	require.Equal(t, http.StatusOK, w.Code)
	require.Empty(t, w.Header().Get("X-Calcifer-Cache"))
	require.Empty(t, w.Header().Get("X-Calcifer-Cache-Similarity"))
	require.Empty(t, w.Header().Get("X-Calcifer-Cache-Timestamp"))
	require.Empty(t, w.Header().Get("X-Calcifer-Cache-Age"))

	// Verify JSON response
	var response domain.CompletionResponse
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	require.Equal(t, "resp-789", response.ID)
	require.InDelta(t, 0.00126, response.Usage.Cost, 0.0001)
}

func TestSetCacheHeaders_NilCacheInfo(t *testing.T) {
	w := httptest.NewRecorder()

	setCacheHeaders(w, nil)

	require.Empty(t, w.Header().Get("X-Calcifer-Cache"))
	require.Empty(t, w.Header().Get("X-Calcifer-Cache-Similarity"))
	require.Empty(t, w.Header().Get("X-Calcifer-Cache-Timestamp"))
	require.Empty(t, w.Header().Get("X-Calcifer-Cache-Age"))
}

func TestSetCacheHeaders_CacheHit(t *testing.T) {
	w := httptest.NewRecorder()
	cachedAt := time.Date(2024, 1, 15, 9, 30, 0, 0, time.UTC)

	cacheInfo := &domain.CacheInfo{
		Hit:             true,
		SimilarityScore: 0.9234,
		CachedAt:        cachedAt,
	}

	setCacheHeaders(w, cacheInfo)

	require.Equal(t, "HIT", w.Header().Get("X-Calcifer-Cache"))
	require.Equal(t, "0.9234", w.Header().Get("X-Calcifer-Cache-Similarity"))
	require.Equal(t, cachedAt.Format(time.RFC3339), w.Header().Get("X-Calcifer-Cache-Timestamp"))

	ageHeader := w.Header().Get("X-Calcifer-Cache-Age")
	require.NotEmpty(t, ageHeader)
}

func TestSetCacheHeaders_CacheMiss(t *testing.T) {
	w := httptest.NewRecorder()

	cacheInfo := &domain.CacheInfo{
		Hit:             false,
		SimilarityScore: 0,
		CachedAt:        time.Time{},
	}

	setCacheHeaders(w, cacheInfo)

	require.Equal(t, "MISS", w.Header().Get("X-Calcifer-Cache"))
	require.Empty(t, w.Header().Get("X-Calcifer-Cache-Similarity"))
	require.Empty(t, w.Header().Get("X-Calcifer-Cache-Timestamp"))
	require.Empty(t, w.Header().Get("X-Calcifer-Cache-Age"))
}

func TestHandleCompletion_MethodNotAllowed(t *testing.T) {
	mockRegistry := mocks.NewMockProviderRegistry(t)
	mockCostCalc := mocks.NewMockCostCalculator(t)
	gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)
	handler := NewHandler(gateway)

	httpReq := httptest.NewRequest(http.MethodGet, "/v1/completions", nil)
	w := httptest.NewRecorder()

	handler.HandleCompletion(w, httpReq)

	require.Equal(t, http.StatusMethodNotAllowed, w.Code)
}

func TestHandleCompletion_InvalidJSON(t *testing.T) {
	mockRegistry := mocks.NewMockProviderRegistry(t)
	mockCostCalc := mocks.NewMockCostCalculator(t)
	gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)
	handler := NewHandler(gateway)

	httpReq := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewReader([]byte("invalid json")))
	w := httptest.NewRecorder()

	handler.HandleCompletion(w, httpReq)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCompletion_MissingModel(t *testing.T) {
	mockRegistry := mocks.NewMockProviderRegistry(t)
	mockCostCalc := mocks.NewMockCostCalculator(t)
	gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)
	handler := NewHandler(gateway)

	req := domain.CompletionRequest{
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
	}

	reqBody, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler.HandleCompletion(w, httpReq)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestHandleCompletion_GatewayError(t *testing.T) {
	mockRegistry := mocks.NewMockProviderRegistry(t)
	mockCostCalc := mocks.NewMockCostCalculator(t)
	mockCache := mocks.NewMockSemanticCache(t)

	gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, mockCache)
	handler := NewHandler(gateway)

	req := domain.CompletionRequest{
		Model: "unknown-model",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: false,
	}

	// Mock cache miss
	mockCache.EXPECT().
		Get(mock.Anything, &req).
		Return(nil, domain.ErrCacheMiss)

	// Mock provider routing failure
	mockRegistry.EXPECT().
		GetByModel(mock.Anything, "unknown-model").
		Return(nil, errors.New("provider not found"))

	reqBody, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler.HandleCompletion(w, httpReq)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleHealth(t *testing.T) {
	mockRegistry := mocks.NewMockProviderRegistry(t)
	mockCostCalc := mocks.NewMockCostCalculator(t)
	gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)
	handler := NewHandler(gateway)

	httpReq := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.HandleHealth(w, httpReq)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var response map[string]string
	err := json.NewDecoder(w.Body).Decode(&response)
	require.NoError(t, err)
	require.Equal(t, "healthy", response["status"])
}

// Streaming Tests

func TestHandleCompletion_Streaming_Success(t *testing.T) {
	mockRegistry := mocks.NewMockProviderRegistry(t)
	mockCostCalc := mocks.NewMockCostCalculator(t)

	gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)
	handler := NewHandler(gateway)

	req := domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}

	// Create mock stream channel
	chunks := make(chan domain.StreamChunk, 3)
	chunks <- domain.StreamChunk{
		Delta: "Hello",
		Done:  false,
		Error: nil,
	}
	chunks <- domain.StreamChunk{
		Delta: " world",
		Done:  false,
		Error: nil,
	}
	chunks <- domain.StreamChunk{
		Delta: "!",
		Done:  true,
		Error: nil,
	}
	close(chunks)

	// Create mock provider and set expectations
	mockProvider := mocks.NewMockProvider(t)
	mockProvider.EXPECT().
		Stream(mock.Anything, &req).
		Return(chunks, nil)

	mockRegistry.EXPECT().
		GetByModel(mock.Anything, "gpt-4").
		Return(mockProvider, nil)

	reqBody, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler.HandleCompletion(w, httpReq)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
	require.Equal(t, "no-cache", w.Header().Get("Cache-Control"))
	require.Equal(t, "keep-alive", w.Header().Get("Connection"))

	// Verify we got SSE formatted output
	body := w.Body.String()
	require.Contains(t, body, "data: ")
	require.Contains(t, body, "Hello")
	require.Contains(t, body, "world")
}

func TestHandleCompletion_Streaming_WithCache(t *testing.T) {
	mockRegistry := mocks.NewMockProviderRegistry(t)
	mockCostCalc := mocks.NewMockCostCalculator(t)
	mockCache := mocks.NewMockSemanticCache(t)

	gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, mockCache)
	handler := NewHandler(gateway)

	req := domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}

	// Create mock stream channel
	chunks := make(chan domain.StreamChunk, 2)
	chunks <- domain.StreamChunk{
		Delta: "Cached response",
		Done:  false,
		Error: nil,
	}
	chunks <- domain.StreamChunk{
		Delta: "",
		Done:  true,
		Error: nil,
	}
	close(chunks)

	// Mock cache miss for streaming (cache doesn't support streaming per design)
	mockCache.EXPECT().
		Get(mock.Anything, &req).
		Return(nil, domain.ErrCacheMiss)

	mockProvider := mocks.NewMockProvider(t)
	mockProvider.EXPECT().
		Stream(mock.Anything, &req).
		Return(chunks, nil)

	mockRegistry.EXPECT().
		GetByModel(mock.Anything, "gpt-4").
		Return(mockProvider, nil)

	// Mock cache set for streaming (buffers the stream)
	mockCache.EXPECT().
		Set(mock.Anything, &req, mock.Anything, mock.Anything).
		Return(nil).
		Maybe()

	reqBody, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler.HandleCompletion(w, httpReq)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))

	// Note: Per design doc, streaming does NOT support cache headers
	require.Empty(t, w.Header().Get("X-Calcifer-Cache"))

	body := w.Body.String()
	require.Contains(t, body, "data: ")
	require.Contains(t, body, "Cached response")
}

func TestHandleCompletion_Streaming_ProviderError(t *testing.T) {
	mockRegistry := mocks.NewMockProviderRegistry(t)
	mockCostCalc := mocks.NewMockCostCalculator(t)

	gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)
	handler := NewHandler(gateway)

	req := domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}

	mockProvider := mocks.NewMockProvider(t)
	mockProvider.EXPECT().
		Stream(mock.Anything, &req).
		Return(nil, errors.New("provider unavailable"))

	mockRegistry.EXPECT().
		GetByModel(mock.Anything, "gpt-4").
		Return(mockProvider, nil)

	reqBody, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler.HandleCompletion(w, httpReq)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestHandleCompletion_Streaming_ChunkError(t *testing.T) {
	mockRegistry := mocks.NewMockProviderRegistry(t)
	mockCostCalc := mocks.NewMockCostCalculator(t)

	gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)
	handler := NewHandler(gateway)

	req := domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}

	// Create mock stream channel with error
	chunks := make(chan domain.StreamChunk, 2)
	chunks <- domain.StreamChunk{
		Delta: "Start",
		Done:  false,
		Error: nil,
	}
	chunks <- domain.StreamChunk{
		Delta: "",
		Done:  false,
		Error: errors.New("stream error occurred"),
	}
	close(chunks)

	mockProvider := mocks.NewMockProvider(t)
	mockProvider.EXPECT().
		Stream(mock.Anything, &req).
		Return(chunks, nil)

	mockRegistry.EXPECT().
		GetByModel(mock.Anything, "gpt-4").
		Return(mockProvider, nil)

	reqBody, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler.HandleCompletion(w, httpReq)

	require.Equal(t, http.StatusOK, w.Code)
	body := w.Body.String()
	require.Contains(t, body, "event: error")
	require.Contains(t, body, "stream error occurred")
}

func TestHandleCompletion_Streaming_ContextCancellation(t *testing.T) {
	mockRegistry := mocks.NewMockProviderRegistry(t)
	mockCostCalc := mocks.NewMockCostCalculator(t)

	gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)
	handler := NewHandler(gateway)

	req := domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}

	// Create mock stream channel that never closes
	chunks := make(chan domain.StreamChunk, 1)
	chunks <- domain.StreamChunk{
		Delta: "Start",
		Done:  false,
		Error: nil,
	}
	// Don't close or send more - simulate long-running stream

	mockProvider := mocks.NewMockProvider(t)
	mockProvider.EXPECT().
		Stream(mock.Anything, &req).
		Return(chunks, nil)

	mockRegistry.EXPECT().
		GetByModel(mock.Anything, "gpt-4").
		Return(mockProvider, nil)

	reqBody, _ := json.Marshal(req)

	// Create request with cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewReader(reqBody))
	httpReq = httpReq.WithContext(ctx)

	w := httptest.NewRecorder()

	// Cancel context after a short delay to simulate client disconnect
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	handler.HandleCompletion(w, httpReq)

	// Should complete without panic/error due to context cancellation
	require.Equal(t, http.StatusOK, w.Code)
}

func TestHandleCompletion_Streaming_EmptyChunks(t *testing.T) {
	mockRegistry := mocks.NewMockProviderRegistry(t)
	mockCostCalc := mocks.NewMockCostCalculator(t)

	gateway := domain.NewGatewayService(mockRegistry, mockCostCalc, nil)
	handler := NewHandler(gateway)

	req := domain.CompletionRequest{
		Model: "gpt-4",
		Messages: []domain.Message{
			{Role: "user", Content: "Hello"},
		},
		Stream: true,
	}

	// Create mock stream channel that closes immediately
	chunks := make(chan domain.StreamChunk)
	close(chunks)

	mockProvider := mocks.NewMockProvider(t)
	mockProvider.EXPECT().
		Stream(mock.Anything, &req).
		Return(chunks, nil)

	mockRegistry.EXPECT().
		GetByModel(mock.Anything, "gpt-4").
		Return(mockProvider, nil)

	reqBody, _ := json.Marshal(req)
	httpReq := httptest.NewRequest(http.MethodPost, "/v1/completions", bytes.NewReader(reqBody))
	w := httptest.NewRecorder()

	handler.HandleCompletion(w, httpReq)

	require.Equal(t, http.StatusOK, w.Code)
	require.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))
}
