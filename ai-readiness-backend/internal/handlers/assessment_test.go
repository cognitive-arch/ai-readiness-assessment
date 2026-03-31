// internal/handlers/assessment_test.go
package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/yourorg/ai-readiness-backend/internal/audit"
	"github.com/yourorg/ai-readiness-backend/internal/models"
	"github.com/yourorg/ai-readiness-backend/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// ─────────────────────────────────────────────
// Mock service (implements AssessmentServicer)
// ─────────────────────────────────────────────

type mockService struct {
	store map[string]*models.Assessment
	bank  *models.QuestionBank
}

func newMockService() *mockService {
	return &mockService{store: make(map[string]*models.Assessment), bank: minimalBank()}
}

func (m *mockService) CreateAssessment(_ context.Context, clientRef string) (*models.Assessment, error) {
	a := &models.Assessment{
		ID: primitive.NewObjectID(), Status: models.StatusDraft,
		Answers: make(map[string]models.Answer), ClientRef: clientRef,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	m.store[a.ID.Hex()] = a
	return a, nil
}

func (m *mockService) GetAssessment(_ context.Context, id string) (*models.Assessment, error) {
	a, ok := m.store[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return a, nil
}

func (m *mockService) SaveAnswers(_ context.Context, id string, answers map[string]models.Answer) (*models.Assessment, error) {
	a, ok := m.store[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	for k, v := range answers {
		a.Answers[k] = v
	}
	a.UpdatedAt = time.Now()
	return a, nil
}

func (m *mockService) ComputeResult(_ context.Context, id string) (*models.Assessment, error) {
	a, ok := m.store[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	a.Result = &models.Result{
		Overall: 72.5, Maturity: "AI Structured", Confidence: 88.9,
		Risks: []string{}, TotalAnswered: 5, TotalQ: 6, ComputedAt: time.Now(),
		DomainScores: map[string]models.DomainScore{
			"Strategic": {Score: 75, Answered: 1, Total: 1},
			"Technology": {Score: 70, Answered: 1, Total: 1},
			"Data": {Score: 65, Answered: 1, Total: 1},
			"Organization": {Score: 80, Answered: 1, Total: 1},
			"Security": {Score: 70, Answered: 1, Total: 1},
			"UseCase": {Score: 75, Answered: 1, Total: 1},
		},
	}
	a.Status = models.StatusComputed
	return a, nil
}

func (m *mockService) GetResult(_ context.Context, id string) (*models.Result, error) {
	a, ok := m.store[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	if a.Result == nil {
		return nil, errors.New("results not yet computed")
	}
	return a.Result, nil
}

func (m *mockService) ListAssessments(_ context.Context, _, _ int64) ([]*models.Assessment, int64, error) {
	var out []*models.Assessment
	for _, a := range m.store {
		out = append(out, a)
	}
	return out, int64(len(out)), nil
}

func (m *mockService) DeleteAssessment(_ context.Context, id string) error {
	if _, ok := m.store[id]; !ok {
		return repository.ErrNotFound
	}
	delete(m.store, id)
	return nil
}

func (m *mockService) QuestionBank() *models.QuestionBank { return m.bank }

// ─────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────

func minimalBank() *models.QuestionBank {
	return &models.QuestionBank{
		Domains: []string{"Strategic", "Technology", "Data", "Organization", "Security", "UseCase"},
		Questions: []models.Question{
			{ID: "s1", Domain: "Strategic", Weight: 3, IsCritical: true},
			{ID: "t1", Domain: "Technology", Weight: 3, IsCritical: true},
			{ID: "d1", Domain: "Data", Weight: 3, IsCritical: true},
			{ID: "o1", Domain: "Organization", Weight: 3, IsCritical: true},
			{ID: "sec1", Domain: "Security", Weight: 3, IsCritical: true},
			{ID: "u1", Domain: "UseCase", Weight: 3, IsCritical: false},
		},
	}
}

func buildRouter(t *testing.T) (*chi.Mux, *mockService) {
	t.Helper()
	log, _ := zap.NewDevelopment()
	svc := newMockService()
	h := NewAssessmentHandler(svc, nil, audit.New(zap.NewNop()), log)
	r := chi.NewRouter()
	r.Get("/api/questions", h.GetQuestions)
	r.Get("/api/assessment", h.List)
	r.Post("/api/assessment", h.Create)
	r.Get("/api/assessment/{id}", h.Get)
	r.Put("/api/assessment/{id}/answers", h.SaveAnswers)
	r.Post("/api/assessment/{id}/compute", h.Compute)
	r.Get("/api/assessment/{id}/results", h.GetResults)
	r.Delete("/api/assessment/{id}", h.Delete)
	return r, svc
}

func postJSON(t *testing.T, router http.Handler, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func doReq(t *testing.T, router http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

func extractID(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var resp apiResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	m, ok := resp.Data.(map[string]interface{})
	if !ok {
		t.Fatalf("data not a map: %T", resp.Data)
	}
	id, _ := m["assessmentId"].(string)
	if id == "" {
		t.Fatal("missing assessmentId")
	}
	return id
}

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

func TestCreateAssessment(t *testing.T) {
	r, _ := buildRouter(t)
	w := postJSON(t, r, "/api/assessment", `{"client_ref":"acme"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateAssessment_EmptyBody(t *testing.T) {
	r, _ := buildRouter(t)
	req := httptest.NewRequest(http.MethodPost, "/api/assessment", bytes.NewBufferString(`{}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Errorf("expected 201 with empty body, got %d", w.Code)
	}
}

func TestGetAssessment_NotFound(t *testing.T) {
	r, _ := buildRouter(t)
	w := doReq(t, r, http.MethodGet, "/api/assessment/"+primitive.NewObjectID().Hex())
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestGetAssessment_InvalidID(t *testing.T) {
	r, _ := buildRouter(t)
	w := doReq(t, r, http.MethodGet, "/api/assessment/not-a-valid-id")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 for invalid ObjectID, got %d", w.Code)
	}
}

func TestFullLifecycle(t *testing.T) {
	r, _ := buildRouter(t)

	// Create
	w := postJSON(t, r, "/api/assessment", `{"client_ref":"test-org"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: expected 201, got %d", w.Code)
	}
	id := extractID(t, w)

	// Get
	w = doReq(t, r, http.MethodGet, "/api/assessment/"+id)
	if w.Code != http.StatusOK {
		t.Fatalf("get: expected 200, got %d", w.Code)
	}

	// Save answers
	body := `{"answers":{"s1":{"score":4},"t1":{"score":3,"comment":"SageMaker"},"d1":{"score":5}}}`
	req := httptest.NewRequest(http.MethodPut, "/api/assessment/"+id+"/answers", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("save answers: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Compute
	w = postJSON(t, r, "/api/assessment/"+id+"/compute", `{}`)
	if w.Code != http.StatusOK {
		t.Fatalf("compute: expected 200, got %d: %s", w.Code, w.Body.String())
	}

	// Get results
	w = doReq(t, r, http.MethodGet, "/api/assessment/"+id+"/results")
	if w.Code != http.StatusOK {
		t.Fatalf("results: expected 200, got %d", w.Code)
	}

	// Delete
	w = doReq(t, r, http.MethodDelete, "/api/assessment/"+id)
	if w.Code != http.StatusOK {
		t.Fatalf("delete: expected 200, got %d", w.Code)
	}

	// Confirm gone
	w = doReq(t, r, http.MethodGet, "/api/assessment/"+id)
	if w.Code != http.StatusNotFound {
		t.Errorf("after delete: expected 404, got %d", w.Code)
	}
}

func TestSaveAnswers_EmptyMap(t *testing.T) {
	r, _ := buildRouter(t)
	w := postJSON(t, r, "/api/assessment", `{}`)
	id := extractID(t, w)

	req := httptest.NewRequest(http.MethodPut, "/api/assessment/"+id+"/answers",
		bytes.NewBufferString(`{"answers":{}}`))
	req.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("empty answers: expected 400, got %d", w2.Code)
	}
}

func TestGetResults_NotComputed(t *testing.T) {
	r, _ := buildRouter(t)
	w := postJSON(t, r, "/api/assessment", `{}`)
	id := extractID(t, w)

	w = doReq(t, r, http.MethodGet, "/api/assessment/"+id+"/results")
	if w.Code != http.StatusConflict {
		t.Errorf("expected 409 when no result, got %d", w.Code)
	}
}

func TestListAssessments(t *testing.T) {
	r, _ := buildRouter(t)
	postJSON(t, r, "/api/assessment", `{"client_ref":"org-a"}`)
	postJSON(t, r, "/api/assessment", `{"client_ref":"org-b"}`)

	w := doReq(t, r, http.MethodGet, "/api/assessment?limit=10&offset=0")
	if w.Code != http.StatusOK {
		t.Fatalf("list: expected 200, got %d", w.Code)
	}
}

func TestGetQuestions(t *testing.T) {
	r, _ := buildRouter(t)
	w := doReq(t, r, http.MethodGet, "/api/questions")
	if w.Code != http.StatusOK {
		t.Fatalf("questions: expected 200, got %d", w.Code)
	}
}

func TestDeleteAssessment_NotFound(t *testing.T) {
	r, _ := buildRouter(t)
	w := doReq(t, r, http.MethodDelete, "/api/assessment/"+primitive.NewObjectID().Hex())
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
