package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/davidbz/calcifer/internal/config"
	"github.com/davidbz/calcifer/internal/observability"
	"go.uber.org/zap"
)

// Server represents the HTTP server.
type Server struct {
	config  config.ServerConfig
	handler *Handler
	logger  *zap.Logger
}

// NewServer creates a new HTTP server.
func NewServer(cfg *config.Config, handler *Handler, logger *zap.Logger) *Server {
	return &Server{
		config:  cfg.Server,
		handler: handler,
		logger:  logger,
	}
}

// Start starts the HTTP server.
func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Register routes.
	mux.HandleFunc("/v1/completions", s.handler.HandleCompletion)
	mux.HandleFunc("/health", s.handler.HandleHealth)

	// Wrap with TraceMiddleware to inject trace IDs into context.
	handlerWithTrace := observability.TraceMiddleware(mux)

	// Create server with timeouts.
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      handlerWithTrace,
		ReadTimeout:  time.Duration(s.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.WriteTimeout) * time.Second,
	}

	s.logger.Info("starting HTTP server", zap.Int("port", s.config.Port))

	return srv.ListenAndServe()
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("shutting down HTTP server")
	return nil
}
