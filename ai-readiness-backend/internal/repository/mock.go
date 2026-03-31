// internal/repository/mock.go
// MockAssessmentRepository is an in-memory implementation of AssessmentRepository.
// Import it in any package that needs to test without MongoDB:
//
//	repo := repository.NewMock()
//	svc  := service.NewAssessmentService(repo, bankPath, log)
package repository

import (
	"context"
	"sync"
	"time"

	"github.com/yourorg/ai-readiness-backend/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// MockAssessmentRepository is a thread-safe in-memory store.
type MockAssessmentRepository struct {
	mu    sync.RWMutex
	store map[string]*models.Assessment
}

// NewMock returns a fresh, empty MockAssessmentRepository.
func NewMock() *MockAssessmentRepository {
	return &MockAssessmentRepository{store: make(map[string]*models.Assessment)}
}

// Len returns the number of documents currently stored.
func (m *MockAssessmentRepository) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.store)
}

// Reset clears all stored documents.
func (m *MockAssessmentRepository) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.store = make(map[string]*models.Assessment)
}

// ─────────────────────────────────────────────
// AssessmentRepository interface implementation
// ─────────────────────────────────────────────

func (m *MockAssessmentRepository) Create(_ context.Context, a *models.Assessment) (*models.Assessment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now().UTC()
	a.ID = primitive.NewObjectID()
	a.CreatedAt = now
	a.UpdatedAt = now
	if a.Status == "" {
		a.Status = models.StatusDraft
	}
	if a.Answers == nil {
		a.Answers = make(map[string]models.Answer)
	}

	// Deep-copy to prevent aliasing
	copied := copyAssessment(a)
	m.store[copied.ID.Hex()] = copied
	return copied, nil
}

func (m *MockAssessmentRepository) GetByID(_ context.Context, id string) (*models.Assessment, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	a, ok := m.store[id]
	if !ok {
		return nil, ErrNotFound
	}
	return copyAssessment(a), nil
}

func (m *MockAssessmentRepository) SaveAnswers(_ context.Context, id string, answers map[string]models.Answer) (*models.Assessment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.store[id]
	if !ok {
		return nil, ErrNotFound
	}

	// Merge answers field by field (mirrors MongoDB $set behaviour)
	for k, v := range answers {
		a.Answers[k] = v
	}
	a.Status = models.StatusInProgress
	a.UpdatedAt = time.Now().UTC()
	return copyAssessment(a), nil
}

func (m *MockAssessmentRepository) SaveResult(_ context.Context, id string, result *models.Result) (*models.Assessment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	a, ok := m.store[id]
	if !ok {
		return nil, ErrNotFound
	}

	result.ComputedAt = time.Now().UTC()
	a.Result = result
	a.Status = models.StatusComputed
	a.UpdatedAt = time.Now().UTC()
	return copyAssessment(a), nil
}

func (m *MockAssessmentRepository) List(_ context.Context, limit, offset int64) ([]*models.Assessment, int64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	all := make([]*models.Assessment, 0, len(m.store))
	for _, a := range m.store {
		all = append(all, copyAssessment(a))
	}

	total := int64(len(all))

	// Apply offset + limit
	if offset >= int64(len(all)) {
		return []*models.Assessment{}, total, nil
	}
	all = all[offset:]
	if limit > 0 && int64(len(all)) > limit {
		all = all[:limit]
	}
	return all, total, nil
}

func (m *MockAssessmentRepository) Delete(_ context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.store[id]; !ok {
		return ErrNotFound
	}
	delete(m.store, id)
	return nil
}

// ─────────────────────────────────────────────
// Deep copy helper — prevents test pollution from shared pointers
// ─────────────────────────────────────────────

func copyAssessment(a *models.Assessment) *models.Assessment {
	cp := *a

	// Deep copy answers map
	if a.Answers != nil {
		cp.Answers = make(map[string]models.Answer, len(a.Answers))
		for k, v := range a.Answers {
			vCopy := v
			if v.Score != nil {
				s := *v.Score
				vCopy.Score = &s
			}
			cp.Answers[k] = vCopy
		}
	}

	// Deep copy result if present
	if a.Result != nil {
		r := *a.Result
		if a.Result.Risks != nil {
			r.Risks = make([]string, len(a.Result.Risks))
			copy(r.Risks, a.Result.Risks)
		}
		if a.Result.Recommendations != nil {
			r.Recommendations = make([]models.Recommendation, len(a.Result.Recommendations))
			copy(r.Recommendations, a.Result.Recommendations)
		}
		if a.Result.DomainScores != nil {
			r.DomainScores = make(map[string]models.DomainScore, len(a.Result.DomainScores))
			for k, v := range a.Result.DomainScores {
				r.DomainScores[k] = v
			}
		}
		cp.Result = &r
	}

	return &cp
}
