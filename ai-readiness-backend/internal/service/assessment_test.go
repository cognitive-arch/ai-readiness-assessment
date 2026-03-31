// internal/service/assessment_test.go
package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/yourorg/ai-readiness-backend/internal/models"
	"github.com/yourorg/ai-readiness-backend/internal/repository"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/zap"
)

// ─────────────────────────────────────────────
// Mock repository
// ─────────────────────────────────────────────

type mockRepo struct {
	store map[string]*models.Assessment
}

func newMockRepo() *mockRepo { return &mockRepo{store: make(map[string]*models.Assessment)} }

func (m *mockRepo) Create(_ context.Context, a *models.Assessment) (*models.Assessment, error) {
	a.ID = primitive.NewObjectID()
	a.CreatedAt = time.Now()
	a.UpdatedAt = time.Now()
	if a.Answers == nil {
		a.Answers = make(map[string]models.Answer)
	}
	m.store[a.ID.Hex()] = a
	return a, nil
}

func (m *mockRepo) GetByID(_ context.Context, id string) (*models.Assessment, error) {
	a, ok := m.store[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	return a, nil
}

func (m *mockRepo) SaveAnswers(_ context.Context, id string, answers map[string]models.Answer) (*models.Assessment, error) {
	a, ok := m.store[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	for k, v := range answers {
		a.Answers[k] = v
	}
	return a, nil
}

func (m *mockRepo) SaveResult(_ context.Context, id string, result *models.Result) (*models.Assessment, error) {
	a, ok := m.store[id]
	if !ok {
		return nil, repository.ErrNotFound
	}
	a.Result = result
	a.Status = models.StatusComputed
	return a, nil
}

func (m *mockRepo) List(_ context.Context, limit, offset int64) ([]*models.Assessment, int64, error) {
	var out []*models.Assessment
	for _, a := range m.store {
		out = append(out, a)
	}
	return out, int64(len(out)), nil
}

func (m *mockRepo) Delete(_ context.Context, id string) error {
	if _, ok := m.store[id]; !ok {
		return repository.ErrNotFound
	}
	delete(m.store, id)
	return nil
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func ptr(i int) *int { return &i }

func makeService(t *testing.T) *AssessmentService {
	t.Helper()
	log, _ := zap.NewDevelopment()
	bank := &models.QuestionBank{
		Domains: []string{"Strategic", "Technology", "Data", "Organization", "Security", "UseCase"},
		Questions: []models.Question{
			{ID: "s1", Domain: "Strategic", Weight: 3, IsCritical: true},
			{ID: "s2", Domain: "Strategic", Weight: 2, IsCritical: false},
			{ID: "t1", Domain: "Technology", Weight: 3, IsCritical: true},
			{ID: "d1", Domain: "Data", Weight: 3, IsCritical: true},
			{ID: "o1", Domain: "Organization", Weight: 3, IsCritical: true},
			{ID: "sec1", Domain: "Security", Weight: 3, IsCritical: true},
			{ID: "u1", Domain: "UseCase", Weight: 3, IsCritical: false},
		},
	}
	return &AssessmentService{repo: newMockRepo(), bank: bank, log: log}
}

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

func TestService_CreateAssessment(t *testing.T) {
	svc := makeService(t)
	ctx := context.Background()

	a, err := svc.CreateAssessment(ctx, "acme")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if a.ID.IsZero() {
		t.Error("expected non-zero ID")
	}
	if a.Status != models.StatusDraft {
		t.Errorf("expected draft, got %s", a.Status)
	}
	if a.ClientRef != "acme" {
		t.Errorf("expected client_ref=acme, got %q", a.ClientRef)
	}
}

func TestService_GetAssessment_NotFound(t *testing.T) {
	svc := makeService(t)
	_, err := svc.GetAssessment(context.Background(), primitive.NewObjectID().Hex())
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestService_SaveAnswers_ValidatesScore(t *testing.T) {
	svc := makeService(t)
	ctx := context.Background()

	a, _ := svc.CreateAssessment(ctx, "")
	id := a.ID.Hex()

	// Score 0 → invalid
	_, err := svc.SaveAnswers(ctx, id, map[string]models.Answer{
		"s1": {Score: ptr(0)},
	})
	if err == nil {
		t.Error("expected error for score=0")
	}

	// Score 6 → invalid
	_, err = svc.SaveAnswers(ctx, id, map[string]models.Answer{
		"s1": {Score: ptr(6)},
	})
	if err == nil {
		t.Error("expected error for score=6")
	}

	// Score 1-5 → valid
	for score := 1; score <= 5; score++ {
		_, err = svc.SaveAnswers(ctx, id, map[string]models.Answer{
			"s1": {Score: ptr(score)},
		})
		if err != nil {
			t.Errorf("score %d should be valid, got error: %v", score, err)
		}
	}
}

func TestService_SaveAnswers_RejectsUnknownQuestions(t *testing.T) {
	svc := makeService(t)
	ctx := context.Background()

	a, _ := svc.CreateAssessment(ctx, "")
	_, err := svc.SaveAnswers(ctx, a.ID.Hex(), map[string]models.Answer{
		"completely_unknown_id": {Score: ptr(3)},
	})
	if err == nil {
		t.Error("expected error for unknown question ID")
	}
}

func TestService_ComputeResult(t *testing.T) {
	svc := makeService(t)
	ctx := context.Background()

	a, _ := svc.CreateAssessment(ctx, "")
	id := a.ID.Hex()

	// Answer all questions
	answers := map[string]models.Answer{
		"s1":   {Score: ptr(4)},
		"s2":   {Score: ptr(4)},
		"t1":   {Score: ptr(4)},
		"d1":   {Score: ptr(4)},
		"o1":   {Score: ptr(4)},
		"sec1": {Score: ptr(4)},
		"u1":   {Score: ptr(4)},
	}
	_, err := svc.SaveAnswers(ctx, id, answers)
	if err != nil {
		t.Fatalf("save answers: %v", err)
	}

	updated, err := svc.ComputeResult(ctx, id)
	if err != nil {
		t.Fatalf("compute: %v", err)
	}
	if updated.Result == nil {
		t.Fatal("result is nil after compute")
	}
	if updated.Status != models.StatusComputed {
		t.Errorf("expected computed status, got %s", updated.Status)
	}

	// Score 4 normalized = ((4-1)/4)*100 = 75
	if updated.Result.Overall < 70 || updated.Result.Overall > 80 {
		t.Errorf("all-4 answers: expected overall ~75, got %.2f", updated.Result.Overall)
	}
}

func TestService_GetResult_NotComputed(t *testing.T) {
	svc := makeService(t)
	ctx := context.Background()

	a, _ := svc.CreateAssessment(ctx, "")
	_, err := svc.GetResult(ctx, a.ID.Hex())
	if err == nil {
		t.Error("expected error when no result computed")
	}
}

func TestService_GetResult_AfterCompute(t *testing.T) {
	svc := makeService(t)
	ctx := context.Background()

	a, _ := svc.CreateAssessment(ctx, "")
	id := a.ID.Hex()

	_, _ = svc.SaveAnswers(ctx, id, map[string]models.Answer{
		"s1": {Score: ptr(3)},
	})
	_, _ = svc.ComputeResult(ctx, id)

	result, err := svc.GetResult(ctx, id)
	if err != nil {
		t.Fatalf("GetResult: %v", err)
	}
	if result.Overall < 0 || result.Overall > 100 {
		t.Errorf("overall out of range: %.2f", result.Overall)
	}
}

func TestService_DeleteAssessment(t *testing.T) {
	svc := makeService(t)
	ctx := context.Background()

	a, _ := svc.CreateAssessment(ctx, "")
	id := a.ID.Hex()

	if err := svc.DeleteAssessment(ctx, id); err != nil {
		t.Fatalf("delete: %v", err)
	}

	_, err := svc.GetAssessment(ctx, id)
	if !errors.Is(err, repository.ErrNotFound) {
		t.Errorf("after delete: expected ErrNotFound, got %v", err)
	}
}

func TestService_ListAssessments(t *testing.T) {
	svc := makeService(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, _ = svc.CreateAssessment(ctx, "")
	}

	list, total, err := svc.ListAssessments(ctx, 10, 0)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total < 3 {
		t.Errorf("expected >= 3 total, got %d", total)
	}
	if len(list) < 3 {
		t.Errorf("expected >= 3 results, got %d", len(list))
	}
}

func TestService_QuestionBank(t *testing.T) {
	svc := makeService(t)
	bank := svc.QuestionBank()
	if bank == nil {
		t.Fatal("QuestionBank returned nil")
	}
	if len(bank.Questions) == 0 {
		t.Error("QuestionBank has no questions")
	}
}
