// internal/metrics/metrics_test.go
package metrics

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMiddleware_RecordsRequestCount(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	// The counter should have at least 1 observation
	count := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("GET", "", "200"))
	if count < 1 {
		t.Errorf("expected ≥1 request counted, got %.0f", count)
	}
}

func TestMiddleware_RecordsDuration(t *testing.T) {
	handler := Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequest(http.MethodPost, "/api/assessment", nil)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	// Histogram should have samples
	count := testutil.CollectAndCount(HTTPRequestDuration)
	if count == 0 {
		t.Fatal("expected histogram samples, got none")
	}
}

func TestTimeScoringOp(t *testing.T) {
	done := TimeScoringOp()
	time.Sleep(1 * time.Millisecond)
	done() // Should record without panic

	val := testutil.ToFloat64(ScoringDuration)
	if val == 0 {
		t.Error("expected scoring duration histogram to have observations")
	}
}

func TestTimeMongoOp(t *testing.T) {
	done := TimeMongoOp("test_op")
	time.Sleep(1 * time.Millisecond)
	done()

	val := testutil.CollectAndCount(MongoOperationDuration)
	if val == 0 {
		t.Error("expected mongo operation histogram to have observations")
	}
}

func TestMetricCounters_NoRace(t *testing.T) {
	// Fire concurrent increments — the race detector will catch issues
	done := make(chan struct{})
	for i := 0; i < 10; i++ {
		go func() {
			AssessmentsCreatedTotal.Inc()
			AnswersSavedTotal.Add(12)
			PDFExportsTotal.WithLabelValues("success").Inc()
			ActiveAssessments.Inc()
			ActiveAssessments.Dec()
			done <- struct{}{}
		}()
	}
	for i := 0; i < 10; i++ {
		<-done
	}
}

func TestHandler_ServesMetricsEndpoint(t *testing.T) {
	// Increment something so the output isn't empty
	AssessmentsCreatedTotal.Inc()

	req := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	w := httptest.NewRecorder()
	Handler().ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	body := w.Body.String()
	if !strings.Contains(body, "aireadiness_assessments_created_total") {
		t.Error("expected aireadiness metric in /metrics output")
	}
	if !strings.Contains(body, "# HELP") {
		t.Error("expected HELP comments in Prometheus output")
	}
}

func TestResponseWriter_CapturesStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := &responseWriter{ResponseWriter: rec, status: http.StatusOK}
	rw.WriteHeader(http.StatusTeapot)
	if rw.status != http.StatusTeapot {
		t.Errorf("expected status 418, got %d", rw.status)
	}
}
