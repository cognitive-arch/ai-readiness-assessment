// internal/metrics/metrics.go
// Prometheus instrumentation for the AI Readiness Assessment API.
// Exposes /metrics endpoint (Prometheus text format).
//
// Collected metrics:
//   - http_requests_total          (counter)   — by method, path, status
//   - http_request_duration_seconds (histogram) — by method, path
//   - assessments_created_total    (counter)
//   - assessments_computed_total   (counter)   — by maturity level
//   - scoring_duration_seconds     (histogram) — scoring engine latency
//   - pdf_exports_total            (counter)   — success / failure
//   - answers_saved_total          (counter)   — total individual answers saved
//   - mongo_operation_duration_sec (histogram) — by operation
//   - active_assessments           (gauge)     — current in-progress count
package metrics

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ─────────────────────────────────────────────
// Metric definitions
// ─────────────────────────────────────────────

var (
	HTTPRequestsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "aireadiness",
			Name:      "http_requests_total",
			Help:      "Total HTTP requests by method, route pattern, and status code.",
		},
		[]string{"method", "route", "status"},
	)

	HTTPRequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "aireadiness",
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency by method and route pattern.",
			Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
		},
		[]string{"method", "route"},
	)

	AssessmentsCreatedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "aireadiness",
			Name:      "assessments_created_total",
			Help:      "Total assessments created.",
		},
	)

	AssessmentsComputedTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "aireadiness",
			Name:      "assessments_computed_total",
			Help:      "Total assessments computed, by resulting maturity level.",
		},
		[]string{"maturity"},
	)

	ScoringDuration = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Namespace: "aireadiness",
			Name:      "scoring_duration_seconds",
			Help:      "Time spent running the scoring engine.",
			Buckets:   []float64{.0001, .0005, .001, .005, .01, .05, .1},
		},
	)

	PDFExportsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "aireadiness",
			Name:      "pdf_exports_total",
			Help:      "Total PDF export attempts by outcome (success|failure).",
		},
		[]string{"outcome"},
	)

	AnswersSavedTotal = promauto.NewCounter(
		prometheus.CounterOpts{
			Namespace: "aireadiness",
			Name:      "answers_saved_total",
			Help:      "Total individual question answers saved across all assessments.",
		},
	)

	MongoOperationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "aireadiness",
			Name:      "mongo_operation_duration_seconds",
			Help:      "MongoDB operation latency by operation name.",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
		},
		[]string{"operation"},
	)

	ActiveAssessments = promauto.NewGauge(
		prometheus.GaugeOpts{
			Namespace: "aireadiness",
			Name:      "active_assessments",
			Help:      "Current number of assessments in in_progress status.",
		},
	)

	RiskFlagsTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "aireadiness",
			Name:      "risk_flags_total",
			Help:      "Total risk flags detected across all computed assessments.",
		},
		[]string{"flag"},
	)
)

// ─────────────────────────────────────────────
// HTTP middleware
// ─────────────────────────────────────────────

// Middleware records HTTP request count and latency for every request.
// Must be used after chi router so chi.RouteContext is populated for route pattern.
func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)

		// Use chi's route pattern (e.g. "/api/assessment/{id}") not the real path
		route := chi.RouteContext(r.Context()).RoutePattern()
		if route == "" {
			route = r.URL.Path
		}

		status := strconv.Itoa(rw.status)
		elapsed := time.Since(start).Seconds()

		HTTPRequestsTotal.WithLabelValues(r.Method, route, status).Inc()
		HTTPRequestDuration.WithLabelValues(r.Method, route).Observe(elapsed)
	})
}

// Handler returns the Prometheus HTTP handler for /metrics.
func Handler() http.Handler {
	return promhttp.Handler()
}

// ─────────────────────────────────────────────
// Timing helpers
// ─────────────────────────────────────────────

// Timer starts a latency measurement for a named Mongo operation.
// Usage: defer metrics.TimeMongoOp("find_by_id")()
func TimeMongoOp(operation string) func() {
	start := time.Now()
	return func() {
		MongoOperationDuration.WithLabelValues(operation).Observe(time.Since(start).Seconds())
	}
}

// TimeScoringOp starts a latency measurement for the scoring engine.
// Usage: defer metrics.TimeScoringOp()()
func TimeScoringOp() func() {
	start := time.Now()
	return func() {
		ScoringDuration.Observe(time.Since(start).Seconds())
	}
}

// ─────────────────────────────────────────────
// responseWriter captures status code
// ─────────────────────────────────────────────

type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.ResponseWriter.WriteHeader(status)
}
