// internal/ratelimit/ratelimit.go
// Per-route rate limiting with configurable limits.
// Uses a token bucket per IP address.
// The global limiter (from go-chi/httprate) applies to all routes.
// This package provides tighter limits for expensive endpoints like /compute and /export/pdf.
package ratelimit

import (
	"net/http"
	"sync"
	"time"
)

// bucket is a simple token bucket for one IP.
type bucket struct {
	tokens   float64
	capacity float64
	rate     float64 // tokens per second
	lastFill time.Time
	mu       sync.Mutex
}

func newBucket(capacity float64, ratePerMinute float64) *bucket {
	return &bucket{
		tokens:   capacity,
		capacity: capacity,
		rate:     ratePerMinute / 60.0,
		lastFill: time.Now(),
	}
}

// take attempts to consume one token. Returns false if the bucket is empty.
func (b *bucket) take() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(b.lastFill).Seconds()
	b.lastFill = now

	b.tokens += elapsed * b.rate
	if b.tokens > b.capacity {
		b.tokens = b.capacity
	}

	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// Limiter manages per-IP token buckets for a single route.
type Limiter struct {
	mu       sync.Mutex
	buckets  map[string]*bucket
	capacity float64
	rpm      float64

	// cleanup
	lastEvict time.Time
	evictTTL  time.Duration
}

// New creates a Limiter allowing rpm requests per minute per IP,
// with a burst capacity of burst tokens.
func New(rpm, burst float64) *Limiter {
	return &Limiter{
		buckets:   make(map[string]*bucket),
		capacity:  burst,
		rpm:       rpm,
		lastEvict: time.Now(),
		evictTTL:  5 * time.Minute,
	}
}

// Middleware returns an http.Handler middleware that rate-limits by remote IP.
// On rate limit: responds 429 with Retry-After header.
func (l *Limiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := extractIP(r)
		if !l.allow(ip) {
			w.Header().Set("Retry-After", "60")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"success":false,"error":"rate limit exceeded, try again in 60 seconds"}`))
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (l *Limiter) allow(ip string) bool {
	l.mu.Lock()
	b, ok := l.buckets[ip]
	if !ok {
		b = newBucket(l.capacity, l.rpm)
		l.buckets[ip] = b
	}
	// Periodic cleanup of stale buckets
	if time.Since(l.lastEvict) > l.evictTTL {
		l.evict()
	}
	l.mu.Unlock()

	return b.take()
}

// evict removes buckets that are full (haven't been used recently).
// Must be called with l.mu held.
func (l *Limiter) evict() {
	for ip, b := range l.buckets {
		b.mu.Lock()
		full := b.tokens >= b.capacity
		b.mu.Unlock()
		if full {
			delete(l.buckets, ip)
		}
	}
	l.lastEvict = time.Now()
}

// extractIP pulls the real client IP, respecting common proxy headers.
func extractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// Take the first IP in the list
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	// Strip port from RemoteAddr
	addr := r.RemoteAddr
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

// ─────────────────────────────────────────────
// Pre-configured limiters for expensive routes
// ─────────────────────────────────────────────

var (
	// ComputeLimiter allows 10 compute calls per minute per IP, burst of 3.
	// Scoring is CPU-intensive; this prevents abuse.
	ComputeLimiter = New(10, 3)

	// PDFLimiter allows 5 PDF exports per minute per IP, burst of 2.
	// PDF generation is I/O and CPU intensive.
	PDFLimiter = New(5, 2)
)
