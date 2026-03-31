// internal/middleware/middleware_test.go
package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"ok":true}`))
}

func TestRequestID_SetsHeader(t *testing.T) {
	handler := RequestID(http.HandlerFunc(okHandler))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	id := w.Header().Get("X-Request-ID")
	if id == "" {
		t.Error("expected X-Request-ID header to be set")
	}
}

func TestRequestID_PreservesExistingID(t *testing.T) {
	handler := RequestID(http.HandlerFunc(okHandler))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "my-custom-id")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if got := w.Header().Get("X-Request-ID"); got != "my-custom-id" {
		t.Errorf("expected preserved request ID, got %q", got)
	}
}

func TestLogger_DoesNotPanic(t *testing.T) {
	log, _ := zap.NewDevelopment()
	handler := Logger(log)(http.HandlerFunc(okHandler))
	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestRecoverer_CatchesPanic(t *testing.T) {
	log, _ := zap.NewDevelopment()
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})
	handler := Recoverer(log)(panicHandler)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	// Must not panic
	handler.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 after panic, got %d", w.Code)
	}
}

func TestResponseWriter_CapturesStatus(t *testing.T) {
	rw := &responseWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK}
	rw.WriteHeader(http.StatusCreated)
	if rw.status != http.StatusCreated {
		t.Errorf("expected captured status 201, got %d", rw.status)
	}
}
