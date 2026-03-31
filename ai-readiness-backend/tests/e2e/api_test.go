//go:build integration

// tests/e2e/api_test.go
// Full end-to-end tests that exercise the real HTTP server against a live MongoDB.
// Run with: go test -tags=integration -v ./tests/e2e/...
// Requires env vars: MONGO_URI, MONGO_DB, QUESTION_BANK_PATH
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	chimiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"

	"github.com/yourorg/ai-readiness-backend/internal/handlers"
	"github.com/yourorg/ai-readiness-backend/internal/repository"
	"github.com/yourorg/ai-readiness-backend/internal/service"
)

// ─────────────────────────────────────────────
// Test harness
// ─────────────────────────────────────────────

type testServer struct {
	server *httptest.Server
	client *http.Client
	db     *mongo.Database
}

func newTestServer(t *testing.T) *testServer {
	t.Helper()

	mongoURI := getEnvOrSkip(t, "MONGO_URI")
	dbName := getenv("MONGO_DB", "ai_readiness_test_"+fmt.Sprintf("%d", time.Now().UnixMilli()))
	bankPath := getenv("QUESTION_BANK_PATH", "../../question-bank-v1.json")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	mongoClient, err := mongo.Connect(ctx, options.Client().ApplyURI(mongoURI))
	if err != nil {
		t.Fatalf("mongo connect: %v", err)
	}
	if err := mongoClient.Ping(ctx, nil); err != nil {
		t.Fatalf("mongo ping: %v", err)
	}

	db := mongoClient.Database(dbName)

	log, _ := zap.NewDevelopment()
	repo, err := repository.NewAssessmentRepository(ctx, db)
	if err != nil {
		t.Fatalf("repo init: %v", err)
	}

	svc, err := service.NewAssessmentService(repo, bankPath, log)
	if err != nil {
		t.Fatalf("service init: %v", err)
	}

	tmpDir := t.TempDir()
	pdfExp := service.NewPDFExporter(tmpDir)
	h := handlers.NewAssessmentHandler(svc, pdfExp, log)

	r := chi.NewRouter()
	r.Use(chimiddleware.Timeout(30 * time.Second))
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
		AllowedHeaders: []string{"Content-Type"},
	}))
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})
	r.Get("/api/questions", h.GetQuestions)
	r.Get("/api/assessment", h.List)
	r.Post("/api/assessment", h.Create)
	r.Get("/api/assessment/{id}", h.Get)
	r.Put("/api/assessment/{id}/answers", h.SaveAnswers)
	r.Post("/api/assessment/{id}/compute", h.Compute)
	r.Get("/api/assessment/{id}/results", h.GetResults)
	r.Get("/api/assessment/{id}/export/pdf", h.ExportPDF)
	r.Delete("/api/assessment/{id}", h.Delete)

	ts := httptest.NewServer(r)

	t.Cleanup(func() {
		ts.Close()
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer dropCancel()
		_ = db.Drop(dropCtx)
		_ = mongoClient.Disconnect(dropCtx)
	})

	return &testServer{
		server: ts,
		client: ts.Client(),
		db:     db,
	}
}

func (ts *testServer) url(path string) string {
	return ts.server.URL + path
}

func (ts *testServer) post(t *testing.T, path, body string) *http.Response {
	t.Helper()
	resp, err := ts.client.Post(ts.url(path), "application/json", bytes.NewBufferString(body))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func (ts *testServer) put(t *testing.T, path, body string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPut, ts.url(path), bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := ts.client.Do(req)
	if err != nil {
		t.Fatalf("PUT %s: %v", path, err)
	}
	return resp
}

func (ts *testServer) get(t *testing.T, path string) *http.Response {
	t.Helper()
	resp, err := ts.client.Get(ts.url(path))
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func (ts *testServer) delete(t *testing.T, path string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodDelete, ts.url(path), nil)
	resp, err := ts.client.Do(req)
	if err != nil {
		t.Fatalf("DELETE %s: %v", path, err)
	}
	return resp
}

// ─────────────────────────────────────────────
// Response helpers
// ─────────────────────────────────────────────

type apiResp struct {
	Success bool                   `json:"success"`
	Data    map[string]interface{} `json:"data"`
	Error   string                 `json:"error"`
	Meta    map[string]interface{} `json:"meta"`
}

type apiRespList struct {
	Success bool                     `json:"success"`
	Data    []map[string]interface{} `json:"data"`
	Meta    map[string]interface{}   `json:"meta"`
}

func decodeResp(t *testing.T, resp *http.Response) apiResp {
	t.Helper()
	defer resp.Body.Close()
	var r apiResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	return r
}

func decodeListResp(t *testing.T, resp *http.Response) apiRespList {
	t.Helper()
	defer resp.Body.Close()
	var r apiRespList
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		t.Fatalf("decode list response: %v", err)
	}
	return r
}

func mustAssessmentID(t *testing.T, r apiResp) string {
	t.Helper()
	id, ok := r.Data["assessmentId"].(string)
	if !ok || id == "" {
		t.Fatalf("missing assessmentId in response: %+v", r.Data)
	}
	return id
}

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

func TestE2E_Health(t *testing.T) {
	ts := newTestServer(t)
	resp := ts.get(t, "/health")
	if resp.StatusCode != http.StatusOK {
		t.Errorf("health: expected 200, got %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestE2E_CreateAndGet(t *testing.T) {
	ts := newTestServer(t)

	// Create
	resp := ts.post(t, "/api/assessment", `{"client_ref":"e2e-test"}`)
	assertStatus(t, resp, http.StatusCreated)
	r := decodeResp(t, resp)
	if !r.Success {
		t.Fatalf("create failed: %s", r.Error)
	}
	id := mustAssessmentID(t, r)
	t.Logf("created assessment: %s", id)

	// Get
	resp = ts.get(t, "/api/assessment/"+id)
	assertStatus(t, resp, http.StatusOK)
	r = decodeResp(t, resp)
	if r.Data["assessmentId"] != id {
		t.Errorf("get: expected id %s, got %v", id, r.Data["assessmentId"])
	}
	if r.Data["status"] != "draft" {
		t.Errorf("expected status=draft, got %v", r.Data["status"])
	}
}

func TestE2E_SaveAnswers_MergesBatches(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.post(t, "/api/assessment", `{}`)
	assertStatus(t, resp, http.StatusCreated)
	id := mustAssessmentID(t, decodeResp(t, resp))

	// First batch — Strategic domain
	batch1 := buildAnswerPayload([]answerEntry{
		{"s1", 4, "Documented strategy exists"},
		{"s2", 3, ""},
		{"s3", 4, ""},
	})
	resp = ts.put(t, "/api/assessment/"+id+"/answers", batch1)
	assertStatus(t, resp, http.StatusOK)

	// Second batch — Technology domain (should not overwrite Strategic)
	batch2 := buildAnswerPayload([]answerEntry{
		{"t1", 5, "SageMaker pipeline"},
		{"t2", 4, "CI/CD in place"},
	})
	resp = ts.put(t, "/api/assessment/"+id+"/answers", batch2)
	assertStatus(t, resp, http.StatusOK)

	// Verify merged state
	resp = ts.get(t, "/api/assessment/"+id)
	assertStatus(t, resp, http.StatusOK)
	r := decodeResp(t, resp)

	answers, ok := r.Data["answers"].(map[string]interface{})
	if !ok {
		t.Fatalf("answers not a map: %T", r.Data["answers"])
	}

	for _, qid := range []string{"s1", "s2", "s3", "t1", "t2"} {
		if _, exists := answers[qid]; !exists {
			t.Errorf("expected answer for %s to be preserved after merge", qid)
		}
	}

	if r.Data["status"] != "in_progress" {
		t.Errorf("expected status=in_progress after answers, got %v", r.Data["status"])
	}
}

func TestE2E_SaveAnswers_InvalidScore(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.post(t, "/api/assessment", `{}`)
	id := mustAssessmentID(t, decodeResp(t, resp))

	// Score 6 is out of range
	resp = ts.put(t, "/api/assessment/"+id+"/answers",
		`{"answers":{"s1":{"score":6}}}`)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestE2E_SaveAnswers_UnknownQuestion(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.post(t, "/api/assessment", `{}`)
	id := mustAssessmentID(t, decodeResp(t, resp))

	resp = ts.put(t, "/api/assessment/"+id+"/answers",
		`{"answers":{"unknown_q_xyz":{"score":3}}}`)
	assertStatus(t, resp, http.StatusBadRequest)
}

func TestE2E_ComputeResult_FullBank(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.post(t, "/api/assessment", `{"client_ref":"full-bank-test"}`)
	id := mustAssessmentID(t, decodeResp(t, resp))

	// Answer all 72 questions with score 3 (defined)
	all := buildAllAnswers(3)
	resp = ts.put(t, "/api/assessment/"+id+"/answers", all)
	assertStatus(t, resp, http.StatusOK)

	// Compute
	resp = ts.post(t, "/api/assessment/"+id+"/compute", `{}`)
	assertStatus(t, resp, http.StatusOK)
	r := decodeResp(t, resp)

	result, ok := r.Data["result"].(map[string]interface{})
	if !ok {
		t.Fatalf("result not present or wrong type: %T", r.Data["result"])
	}

	// Score 3 normalized = ((3-1)/4)*100 = 50 → overall should be ~50
	overall := result["overall"].(float64)
	if overall < 45 || overall > 55 {
		t.Errorf("all-3 answers: expected overall ~50, got %.2f", overall)
	}

	if result["maturity"] != "AI Emerging" {
		t.Errorf("expected AI Emerging maturity for ~50 score, got %v", result["maturity"])
	}

	confidence := result["confidence"].(float64)
	if confidence != 100.0 {
		t.Errorf("expected 100%% confidence when all answered, got %.2f", confidence)
	}

	if r.Data["status"] != "computed" {
		t.Errorf("expected status=computed, got %v", r.Data["status"])
	}
}

func TestE2E_ComputeResult_CriticalGapsRisk(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.post(t, "/api/assessment", `{}`)
	id := mustAssessmentID(t, decodeResp(t, resp))

	// Answer all questions with 4, but set critical s1 and s2 to 2
	all := buildAllAnswersMap(4)
	all["s1"] = 2
	all["s2"] = 2
	resp = ts.put(t, "/api/assessment/"+id+"/answers", marshalAnswers(all))
	assertStatus(t, resp, http.StatusOK)

	resp = ts.post(t, "/api/assessment/"+id+"/compute", `{}`)
	assertStatus(t, resp, http.StatusOK)
	r := decodeResp(t, resp)

	result := r.Data["result"].(map[string]interface{})
	risks := toStringSlice(result["risks"])
	if !contains(risks, "CRITICAL_GAPS") {
		t.Errorf("expected CRITICAL_GAPS risk, got: %v", risks)
	}
}

func TestE2E_ComputeResult_DataHighRisk(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.post(t, "/api/assessment", `{}`)
	id := mustAssessmentID(t, decodeResp(t, resp))

	all := buildAllAnswersMap(4)
	// Drive all Data domain questions to score 1 → domain score = 0
	for _, qid := range []string{"d1","d2","d3","d4","d5","d6","d7","d8","d9","d10","d11","d12"} {
		all[qid] = 1
	}
	resp = ts.put(t, "/api/assessment/"+id+"/answers", marshalAnswers(all))
	assertStatus(t, resp, http.StatusOK)

	resp = ts.post(t, "/api/assessment/"+id+"/compute", `{}`)
	assertStatus(t, resp, http.StatusOK)
	r := decodeResp(t, resp)
	result := r.Data["result"].(map[string]interface{})
	risks := toStringSlice(result["risks"])
	if !contains(risks, "DATA_HIGH_RISK") {
		t.Errorf("expected DATA_HIGH_RISK, got: %v", risks)
	}
}

func TestE2E_ComputeResult_MaturityCapped(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.post(t, "/api/assessment", `{}`)
	id := mustAssessmentID(t, decodeResp(t, resp))

	// All 5 (would give AI-Native) but Data = 1 → should cap to AI Structured
	all := buildAllAnswersMap(5)
	for _, qid := range []string{"d1","d2","d3","d4","d5","d6","d7","d8","d9","d10","d11","d12"} {
		all[qid] = 1
	}
	resp = ts.put(t, "/api/assessment/"+id+"/answers", marshalAnswers(all))
	assertStatus(t, resp, http.StatusOK)

	resp = ts.post(t, "/api/assessment/"+id+"/compute", `{}`)
	assertStatus(t, resp, http.StatusOK)
	r := decodeResp(t, resp)
	result := r.Data["result"].(map[string]interface{})

	if result["maturity"] != "AI Structured" {
		t.Errorf("expected maturity capped at AI Structured, got %v", result["maturity"])
	}
	risks := toStringSlice(result["risks"])
	if !contains(risks, "MATURITY_CAPPED") {
		t.Errorf("expected MATURITY_CAPPED risk, got: %v", risks)
	}
}

func TestE2E_GetResults_BeforeCompute(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.post(t, "/api/assessment", `{}`)
	id := mustAssessmentID(t, decodeResp(t, resp))

	resp = ts.get(t, "/api/assessment/"+id+"/results")
	assertStatus(t, resp, http.StatusConflict)
	resp.Body.Close()
}

func TestE2E_GetResults_AfterCompute(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.post(t, "/api/assessment", `{}`)
	id := mustAssessmentID(t, decodeResp(t, resp))

	resp = ts.put(t, "/api/assessment/"+id+"/answers", buildAllAnswers(3))
	assertStatus(t, resp, http.StatusOK)

	resp = ts.post(t, "/api/assessment/"+id+"/compute", `{}`)
	assertStatus(t, resp, http.StatusOK)

	// GET /results returns just the result object
	resp = ts.get(t, "/api/assessment/"+id+"/results")
	assertStatus(t, resp, http.StatusOK)
	r := decodeResp(t, resp)
	if r.Data["overall"] == nil {
		t.Error("expected overall score in results response")
	}
}

func TestE2E_ExportPDF(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.post(t, "/api/assessment", `{}`)
	id := mustAssessmentID(t, decodeResp(t, resp))

	// PDF before compute → 409
	resp = ts.get(t, "/api/assessment/"+id+"/export/pdf")
	assertStatus(t, resp, http.StatusConflict)
	resp.Body.Close()

	// Compute first
	resp = ts.put(t, "/api/assessment/"+id+"/answers", buildAllAnswers(4))
	assertStatus(t, resp, http.StatusOK)
	resp = ts.post(t, "/api/assessment/"+id+"/compute", `{}`)
	assertStatus(t, resp, http.StatusOK)

	// Now export
	resp = ts.get(t, "/api/assessment/"+id+"/export/pdf")
	assertStatus(t, resp, http.StatusOK)
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct != "application/pdf" {
		t.Errorf("expected Content-Type application/pdf, got %s", ct)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) < 100 {
		t.Errorf("PDF body suspiciously small: %d bytes", len(body))
	}
	// PDF magic bytes
	if string(body[:4]) != "%PDF" {
		t.Errorf("response does not start with %%PDF header")
	}
}

func TestE2E_List_Pagination(t *testing.T) {
	ts := newTestServer(t)

	// Create 5 assessments
	for i := 0; i < 5; i++ {
		resp := ts.post(t, "/api/assessment", fmt.Sprintf(`{"client_ref":"org-%d"}`, i))
		assertStatus(t, resp, http.StatusCreated)
		resp.Body.Close()
	}

	// Fetch page 1 of 2
	resp := ts.get(t, "/api/assessment?limit=3&offset=0")
	assertStatus(t, resp, http.StatusOK)
	lr := decodeListResp(t, resp)
	if len(lr.Data) > 3 {
		t.Errorf("limit=3: expected ≤3 results, got %d", len(lr.Data))
	}
	total := lr.Meta["total"].(float64)
	if total < 5 {
		t.Errorf("expected total >= 5, got %.0f", total)
	}
}

func TestE2E_Delete(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.post(t, "/api/assessment", `{}`)
	id := mustAssessmentID(t, decodeResp(t, resp))

	resp = ts.delete(t, "/api/assessment/"+id)
	assertStatus(t, resp, http.StatusOK)
	resp.Body.Close()

	resp = ts.get(t, "/api/assessment/"+id)
	assertStatus(t, resp, http.StatusNotFound)
	resp.Body.Close()
}

func TestE2E_GetQuestions(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.get(t, "/api/questions")
	assertStatus(t, resp, http.StatusOK)
	r := decodeResp(t, resp)

	questions, ok := r.Data["questions"].([]interface{})
	if !ok {
		t.Fatalf("questions field missing or wrong type")
	}
	if len(questions) != 72 {
		t.Errorf("expected 72 questions, got %d", len(questions))
	}
}

func TestE2E_ConcurrentAnswerSaves(t *testing.T) {
	ts := newTestServer(t)

	resp := ts.post(t, "/api/assessment", `{}`)
	id := mustAssessmentID(t, decodeResp(t, resp))

	// Fire 6 concurrent domain-level saves
	domains := []struct {
		qids   []string
		scores []int
	}{
		{[]string{"s1", "s2"}, []int{4, 3}},
		{[]string{"t1", "t2"}, []int{5, 4}},
		{[]string{"d1", "d2"}, []int{3, 4}},
		{[]string{"o1", "o2"}, []int{4, 4}},
		{[]string{"sec1", "sec2"}, []int{3, 3}},
		{[]string{"u1", "u2"}, []int{5, 4}},
	}

	errc := make(chan error, len(domains))
	for _, d := range domains {
		d := d
		go func() {
			entries := make([]answerEntry, len(d.qids))
			for i, qid := range d.qids {
				entries[i] = answerEntry{qid, d.scores[i], ""}
			}
			r, err := ts.client.Do(func() *http.Request {
				req, _ := http.NewRequest(http.MethodPut,
					ts.url("/api/assessment/"+id+"/answers"),
					bytes.NewBufferString(buildAnswerPayload(entries)))
				req.Header.Set("Content-Type", "application/json")
				return req
			}())
			if err != nil {
				errc <- err
				return
			}
			r.Body.Close()
			if r.StatusCode != http.StatusOK {
				errc <- fmt.Errorf("concurrent save: unexpected status %d", r.StatusCode)
				return
			}
			errc <- nil
		}()
	}

	for range domains {
		if err := <-errc; err != nil {
			t.Errorf("concurrent save error: %v", err)
		}
	}
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

type answerEntry struct {
	QID     string
	Score   int
	Comment string
}

func buildAnswerPayload(entries []answerEntry) string {
	m := map[string]interface{}{}
	for _, e := range entries {
		a := map[string]interface{}{"score": e.Score}
		if e.Comment != "" {
			a["comment"] = e.Comment
		}
		m[e.QID] = a
	}
	b, _ := json.Marshal(map[string]interface{}{"answers": m})
	return string(b)
}

func buildAllAnswers(score int) string {
	all := buildAllAnswersMap(score)
	return marshalAnswers(all)
}

func buildAllAnswersMap(score int) map[string]int {
	qids := allQuestionIDs()
	m := make(map[string]int, len(qids))
	for _, qid := range qids {
		m[qid] = score
	}
	return m
}

func marshalAnswers(m map[string]int) string {
	answers := make(map[string]interface{}, len(m))
	for qid, score := range m {
		answers[qid] = map[string]interface{}{"score": score}
	}
	b, _ := json.Marshal(map[string]interface{}{"answers": answers})
	return string(b)
}

func allQuestionIDs() []string {
	return []string{
		"s1","s2","s3","s4","s5","s6","s7","s8","s9","s10","s11","s12",
		"t1","t2","t3","t4","t5","t6","t7","t8","t9","t10","t11","t12",
		"d1","d2","d3","d4","d5","d6","d7","d8","d9","d10","d11","d12",
		"o1","o2","o3","o4","o5","o6","o7","o8","o9","o10","o11","o12",
		"sec1","sec2","sec3","sec4","sec5","sec6","sec7","sec8","sec9","sec10","sec11","sec12",
		"u1","u2","u3","u4","u5","u6","u7","u8","u9","u10","u11","u12",
	}
}

func assertStatus(t *testing.T, resp *http.Response, expected int) {
	t.Helper()
	if resp.StatusCode != expected {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		t.Fatalf("expected HTTP %d, got %d\nbody: %s", expected, resp.StatusCode, string(body))
	}
}

func toStringSlice(v interface{}) []string {
	raw, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		if s, ok := r.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func contains(ss []string, target string) bool {
	for _, s := range ss {
		if s == target {
			return true
		}
	}
	return false
}

func getEnvOrSkip(t *testing.T, key string) string {
	t.Helper()
	v := os.Getenv(key)
	if v == "" {
		t.Skipf("skipping: %s not set", key)
	}
	return v
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Suppress unused import for bson (used in cleanup via db.Drop)
var _ = bson.M{}
