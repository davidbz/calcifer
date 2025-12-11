package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"

	"github.com/davidbz/calcifer/internal/config"
	"github.com/davidbz/calcifer/internal/http/middleware"
)

// Server represents the HTTP server.
type Server struct {
	config      config.ServerConfig
	handler     *Handler
	middlewares middleware.Middleware
	logger      *zap.Logger
}

// NewServer creates a new HTTP server.
func NewServer(
	cfg *config.Config,
	handler *Handler,
	middlewares middleware.Middleware,
	logger *zap.Logger,
) *Server {
	return &Server{
		config:      cfg.Server,
		handler:     handler,
		middlewares: middlewares,
		logger:      logger,
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
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      handlerWithMiddleware,
		ReadTimeout:  time.Duration(s.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.WriteTimeout) * time.Second,
	}

	s.logger.Info("starting HTTP server", zap.Int("port", s.config.Port))

	if err := srv.ListenAndServe(); err != nil {
		return fmt.Errorf("server failed: %w", err)
	}
	return nil
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(_ context.Context) error {
	s.logger.Info("shutting down HTTP server")
	return nil
}
