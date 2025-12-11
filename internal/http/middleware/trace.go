package middleware

import (
	"net/http"

	"github.com/davidbz/calcifer/internal/observability"
)

// Trace creates a middleware that injects trace ID and request ID into every request.
func Trace() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			traceID := observability.GenerateTraceID()
			ctx = observability.WithTraceID(ctx, traceID)

			spanID := observability.GenerateSpanID()
			ctx = observability.WithSpanID(ctx, spanID)

			requestID := observability.GenerateRequestID()
			ctx = observability.WithRequestID(ctx, requestID)

			w.Header().Set("X-Trace-Id", traceID)
			w.Header().Set("X-Request-Id", requestID)

			contextLogger := observability.FromContext(ctx)
			contextLogger.Info("request started",
				observability.String("method", r.Method),
				observability.String("path", r.URL.Path),
				observability.String("remote_addr", r.RemoteAddr),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
