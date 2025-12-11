package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/davidbz/calcifer/internal/config"
)

// Server represents the HTTP server.
type Server struct {
	config  config.ServerConfig
	handler *Handler
	logger  *slog.Logger
}

// NewServer creates a new HTTP server.
func NewServer(cfg *config.Config, handler *Handler, logger *slog.Logger) *Server {
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

	// Create server with timeouts.
	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", s.config.Port),
		Handler:      s.loggingMiddleware(mux),
		ReadTimeout:  time.Duration(s.config.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(s.config.WriteTimeout) * time.Second,
	}

	s.logger.Info("starting HTTP server", "port", s.config.Port)

	return srv.ListenAndServe()
}

// loggingMiddleware logs HTTP requests.
func (s *Server) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		s.logger.InfoContext(r.Context(), "request started",
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)

		next.ServeHTTP(w, r)

		s.logger.InfoContext(r.Context(), "request completed",
			"method", r.Method,
			"path", r.URL.Path,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// Shutdown gracefully shuts down the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.InfoContext(ctx, "shutting down HTTP server")
	return nil
}
