package middleware

import (
	"net/http"

	"github.com/rs/cors"

	"github.com/davidbz/calcifer/internal/config"
)

// CORS creates a middleware that handles Cross-Origin Resource Sharing (CORS)
// using the github.com/rs/cors library.
func CORS(cfg *config.CORSConfig) Middleware {
	if cfg == nil {
		// Return no-op middleware if config is nil.
		return func(next http.Handler) http.Handler {
			return next
		}
	}

	c := cors.New(cors.Options{
		AllowedOrigins:   cfg.AllowedOrigins,
		AllowedMethods:   cfg.AllowedMethods,
		AllowedHeaders:   cfg.AllowedHeaders,
		AllowCredentials: cfg.AllowCredentials,
		MaxAge:           cfg.MaxAge,
	})

	return func(next http.Handler) http.Handler {
		return c.Handler(next)
	}
}
