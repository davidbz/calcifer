package middleware

import (
	"net/http"

	"github.com/davidbz/calcifer/internal/config"
)

// Middleware wraps an http.Handler with additional functionality.
// Middlewares can be composed using the Chain function.
type Middleware func(http.Handler) http.Handler

// Chain composes multiple middlewares into a single middleware.
// Middlewares are applied in the order they are provided, with the first
// middleware being the outermost wrapper (executed first on request).
//
// Example:
//
//	chain := Chain(CORS(corsConfig), Trace(), Auth(authConfig))
//	handler := chain(mux)
func Chain(middlewares ...Middleware) Middleware {
	return func(final http.Handler) http.Handler {
		// Apply in reverse order so first middleware wraps outermost.
		for i := len(middlewares) - 1; i >= 0; i-- {
			final = middlewares[i](final)
		}
		return final
	}
}

// BuildMiddlewareChain composes the middleware chain for production.
// Order matters: CORS -> Trace.
func BuildMiddlewareChain(corsConfig *config.CORSConfig) Middleware {
	return Chain(
		CORS(corsConfig),
		Trace(),
	)
}
