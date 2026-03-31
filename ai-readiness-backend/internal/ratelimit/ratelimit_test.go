// internal/ratelimit/ratelimit_test.go
package ratelimit

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// ─────────────────────────────────────────────
// Bucket unit tests
// ─────────────────────────────────────────────

func TestBucket_StartsFullAndDrains(t *testing.T) {
	b := newBucket(5, 60) // 5 burst, 1/s refill

	// Should be able to take 5 tokens immediately
	for i := 0; i < 5; i++ {
		if !b.take() {
			t.Errorf("take %d: expected true (burst), got false", i+1)
		}
	}

	// 6th should fail
	if b.take() {
		t.Error("expected false after burst exhausted")
	}
}

func TestBucket_RefillsOverTime(t *testing.T) {
	b := newBucket(1, 600) // 1 burst, 10/s refill
	b.take()               // drain the one token

	// Should be empty
	if b.take() {
		t.Error("should be empty immediately after drain")
	}

	// Manually simulate 200ms passing (gets 2 tokens at 10/s)
	b.mu.Lock()
	b.lastFill = time.Now().Add(-200 * time.Millisecond)
	b.mu.Unlock()

	if !b.take() {
		t.Error("expected token after 200ms at 10/s refill rate")
	}
}

func TestBucket_DoesNotExceedCapacity(t *testing.T) {
	b := newBucket(3, 600) // 3 burst, 10/s refill

	// Simulate 1 hour passing — tokens should cap at capacity
	b.mu.Lock()
	b.lastFill = time.Now().Add(-1 * time.Hour)
	b.mu.Unlock()

	b.take() // trigger refill
	b.mu.Lock()
	if b.tokens > b.capacity {
		t.Errorf("tokens %.2f exceeded capacity %.2f", b.tokens, b.capacity)
	}
	b.mu.Unlock()
}

// ─────────────────────────────────────────────
// Limiter middleware tests
// ─────────────────────────────────────────────

func okHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

func TestLimiter_AllowsUnderLimit(t *testing.T) {
	l := New(60, 5) // 60 rpm, 5 burst
	handler := l.Middleware(http.HandlerFunc(okHandler))

	for i := 0; i < 5; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = "192.168.1.1:9999"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("request %d: expected 200, got %d", i+1, w.Code)
		}
	}
}

func TestLimiter_Returns429WhenExhausted(t *testing.T) {
	l := New(60, 2) // 2 burst only
	handler := l.Middleware(http.HandlerFunc(okHandler))

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/", nil)
		req.RemoteAddr = "10.0.0.1:1234"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
	}

	// 3rd request should be rate limited
	req := httptest.NewRequest(http.MethodPost, "/", nil)
	req.RemoteAddr = "10.0.0.1:1234"
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusTooManyRequests {
		t.Errorf("expected 429, got %d", w.Code)
	}
	if ra := w.Header().Get("Retry-After"); ra == "" {
		t.Error("expected Retry-After header on 429 response")
	}
	body := w.Body.String()
	if !strings.Contains(body, "rate limit exceeded") {
		t.Errorf("expected rate limit message in body, got: %s", body)
	}
}

func TestLimiter_DifferentIPsHaveSeparateBuckets(t *testing.T) {
	l := New(60, 1) // 1 burst per IP
	handler := l.Middleware(http.HandlerFunc(okHandler))

	ips := []string{"1.1.1.1:0", "2.2.2.2:0", "3.3.3.3:0"}
	for _, ip := range ips {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = ip
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("ip %s: expected 200, got %d", ip, w.Code)
		}
	}
}

func TestLimiter_SameIPSecondRequestBlocked(t *testing.T) {
	l := New(60, 1)
	handler := l.Middleware(http.HandlerFunc(okHandler))

	for i, expected := range []int{http.StatusOK, http.StatusTooManyRequests} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.RemoteAddr = "5.5.5.5:0"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != expected {
			t.Errorf("request %d: expected %d, got %d", i+1, expected, w.Code)
		}
	}
}

// ─────────────────────────────────────────────
// IP extraction tests
// ─────────────────────────────────────────────

func TestExtractIP_FromRemoteAddr(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "192.168.1.100:54321"
	if got := extractIP(r); got != "192.168.1.100" {
		t.Errorf("expected 192.168.1.100, got %q", got)
	}
}

func TestExtractIP_FromXForwardedFor(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:80"
	r.Header.Set("X-Forwarded-For", "203.0.113.1, 10.0.0.2, 10.0.0.3")
	if got := extractIP(r); got != "203.0.113.1" {
		t.Errorf("expected 203.0.113.1 (first in XFF), got %q", got)
	}
}

func TestExtractIP_FromXRealIP(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = "10.0.0.1:80"
	r.Header.Set("X-Real-IP", "198.51.100.42")
	if got := extractIP(r); got != "198.51.100.42" {
		t.Errorf("expected 198.51.100.42, got %q", got)
	}
}

func TestExtractIP_XForwardedForSingleValue(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", "203.0.113.55")
	if got := extractIP(r); got != "203.0.113.55" {
		t.Errorf("expected 203.0.113.55, got %q", got)
	}
}

func TestExtractIP_XFFTakesPrecedenceOverXRI(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("X-Forwarded-For", "1.2.3.4")
	r.Header.Set("X-Real-IP", "5.6.7.8")
	if got := extractIP(r); got != "1.2.3.4" {
		t.Errorf("expected XFF to take precedence, got %q", got)
	}
}

// ─────────────────────────────────────────────
// Pre-configured limiters smoke test
// ─────────────────────────────────────────────

func TestPreConfiguredLimiters_NotNil(t *testing.T) {
	if ComputeLimiter == nil {
		t.Error("ComputeLimiter should not be nil")
	}
	if PDFLimiter == nil {
		t.Error("PDFLimiter should not be nil")
	}
}

func TestComputeLimiter_AllowsBurst(t *testing.T) {
	l := New(10, 3) // mirrors ComputeLimiter config
	handler := l.Middleware(http.HandlerFunc(okHandler))

	allowed := 0
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodPost, "/compute", nil)
		req.RemoteAddr = "9.9.9.9:0"
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			allowed++
		}
	}
	if allowed != 3 {
		t.Errorf("expected exactly 3 allowed (burst=3), got %d", allowed)
	}
}

// ─────────────────────────────────────────────
// Concurrency safety test
// ─────────────────────────────────────────────

func TestLimiter_ConcurrentAccessNoRace(t *testing.T) {
	l := New(1000, 500)
	handler := l.Middleware(http.HandlerFunc(okHandler))

	done := make(chan struct{}, 50)
	for i := 0; i < 50; i++ {
		go func(n int) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = "127.0.0.1:0"
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			done <- struct{}{}
		}(i)
	}
	for i := 0; i < 50; i++ {
		<-done
	}
}
