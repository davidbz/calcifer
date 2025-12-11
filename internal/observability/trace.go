package observability

import (
	"net/http"

	"go.uber.org/zap"
)

// Trace creates a middleware that injects trace ID and request ID into every request.
func Trace() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			traceID := GenerateTraceID()
			ctx = WithTraceID(ctx, traceID)

			spanID := GenerateSpanID()
			ctx = WithSpanID(ctx, spanID)

			requestID := GenerateRequestID()
			ctx = WithRequestID(ctx, requestID)

			w.Header().Set("X-Trace-Id", traceID)
			w.Header().Set("X-Request-Id", requestID)

			contextLogger := FromContext(ctx)
			contextLogger.Info("request started",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("remote_addr", r.RemoteAddr),
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
