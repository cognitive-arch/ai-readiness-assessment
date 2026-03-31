// internal/middleware/middleware.go
package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const requestIDHeader = "X-Request-ID"

// RequestID injects a unique request ID into the response header and request context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(requestIDHeader)
		if id == "" {
			id = uuid.New().String()
		}
		w.Header().Set(requestIDHeader, id)
		next.ServeHTTP(w, r)
	})
}

// Logger logs method, path, status, latency, and request ID for every request.
func Logger(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rw, r)

			log.Info("http request",
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", rw.status),
				zap.Duration("latency", time.Since(start)),
				zap.String("request_id", w.Header().Get(requestIDHeader)),
				zap.String("remote_addr", r.RemoteAddr),
			)
		})
	}
}

// Recoverer catches panics, logs them, and returns 500.
func Recoverer(log *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					log.Error("panic recovered",
						zap.String("panic", fmt.Sprintf("%v", rec)),
						zap.String("stack", string(debug.Stack())),
					)
					http.Error(w, `{"success":false,"error":"internal server error"}`, http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}
