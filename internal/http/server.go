package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/davidbz/calcifer/internal/config"
	"github.com/davidbz/calcifer/internal/http/middleware"
	"github.com/davidbz/calcifer/internal/observability"
)

// Server represents the HTTP server.
type Server struct {
	config      config.ServerConfig
	handler     *Handler
	middlewares middleware.Middleware
	srv         *http.Server
}

// NewServer creates a new HTTP server.
func NewServer(
	cfg *config.Config,
	handler *Handler,
	middlewares middleware.Middleware,
) *Server {
	return &Server{
		config:      cfg.Server,
		handler:     handler,
		middlewares: middlewares,
		srv:         nil,
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Register routes.
	mux.HandleFunc("/v1/completions", s.handler.HandleCompletion)
	mux.HandleFunc("/health", s.handler.HandleHealth)

	// Apply middleware chain.
	handlerWithMiddleware := s.middlewares(mux)

	// Create server with timeouts.
	s.srv = &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      handlerWithMiddleware,
		ReadTimeout:  time.Duration(s.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.WriteTimeout) * time.Second,
	}

	ctx := context.Background()
	observability.FromContext(ctx).Info("starting HTTP server", observability.Int("port", s.config.Port))

	if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	observability.FromContext(ctx).Info("shutting down HTTP server")

	if s.srv == nil {
		return nil
	}

	if err := s.srv.Shutdown(ctx); err != nil {
		return fmt.Errorf("failed to shutdown server: %w", err)
	}

	return nil
}
