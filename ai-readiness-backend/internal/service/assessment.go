// internal/service/assessment.go
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/yourorg/ai-readiness-backend/internal/models"
	"github.com/yourorg/ai-readiness-backend/internal/repository"
	"github.com/yourorg/ai-readiness-backend/internal/scoring"
	"go.uber.org/zap"
)

// AssessmentService encapsulates all business logic for assessments.
type AssessmentService struct {
	repo     repository.AssessmentRepository
	bank     *models.QuestionBank
	log      *zap.Logger
}

// NewAssessmentService constructs the service and loads the question bank from disk.
func NewAssessmentService(
	repo repository.AssessmentRepository,
	bankPath string,
	log *zap.Logger,
) (*AssessmentService, error) {
	bank, err := loadQuestionBank(bankPath)
	if err != nil {
		return nil, fmt.Errorf("load question bank: %w", err)
	}
	log.Info("question bank loaded",
		zap.Int("questions", len(bank.Questions)),
		zap.Strings("domains", bank.Domains),
	)
	return &AssessmentService{repo: repo, bank: bank, log: log}, nil
}

// QuestionBank returns the loaded question bank (read-only).
func (s *AssessmentService) QuestionBank() *models.QuestionBank { return s.bank }

// CreateAssessment creates a new, empty assessment document.
func (s *AssessmentService) CreateAssessment(ctx context.Context, clientRef string) (*models.Assessment, error) {
	a := &models.Assessment{
		Status:    models.StatusDraft,
		Answers:   make(map[string]models.Answer),
		ClientRef: clientRef,
	}
	created, err := s.repo.Create(ctx, a)
	if err != nil {
		return nil, fmt.Errorf("create assessment: %w", err)
	}
	s.log.Info("assessment created", zap.String("id", created.ID.Hex()))
	return created, nil
}

// GetAssessment retrieves an assessment by ID.
func (s *AssessmentService) GetAssessment(ctx context.Context, id string) (*models.Assessment, error) {
	return s.repo.GetByID(ctx, id)
}

// SaveAnswers merges a batch of answers into the assessment.
// Validates that all question IDs belong to the known question bank.
func (s *AssessmentService) SaveAnswers(
	ctx context.Context,
	id string,
	answers map[string]models.Answer,
) (*models.Assessment, error) {
	// Validate question IDs and score range
	validIDs := s.validQuestionIDs()
	for qID, ans := range answers {
		if _, ok := validIDs[qID]; !ok {
			return nil, fmt.Errorf("unknown question ID %q", qID)
		}
		if ans.Score != nil && (*ans.Score < 1 || *ans.Score > 5) {
			return nil, fmt.Errorf("score for question %q must be 1-5, got %d", qID, *ans.Score)
		}
	}

	updated, err := s.repo.SaveAnswers(ctx, id, answers)
	if err != nil {
		return nil, err
	}
	s.log.Info("answers saved",
		zap.String("id", id),
		zap.Int("count", len(answers)),
	)
	return updated, nil
}

// ComputeResult runs the scoring engine on all answers stored in the assessment,
// persists the result, and returns the updated assessment.
func (s *AssessmentService) ComputeResult(ctx context.Context, id string) (*models.Assessment, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	result := scoring.Compute(s.bank, a.Answers)
	updated, err := s.repo.SaveResult(ctx, id, result)
	if err != nil {
		return nil, fmt.Errorf("persist result: %w", err)
	}

	s.log.Info("result computed",
		zap.String("id", id),
		zap.Float64("overall", result.Overall),
		zap.String("maturity", result.Maturity),
		zap.Float64("confidence", result.Confidence),
	)
	return updated, nil
}

// GetResult returns the result if available, or an error if not yet computed.
func (s *AssessmentService) GetResult(ctx context.Context, id string) (*models.Result, error) {
	a, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if a.Result == nil {
		return nil, fmt.Errorf("results not yet computed for assessment %s", id)
	}
	return a.Result, nil
}

// ListAssessments returns paginated assessments.
func (s *AssessmentService) ListAssessments(ctx context.Context, limit, offset int64) ([]*models.Assessment, int64, error) {
	return s.repo.List(ctx, limit, offset)
}

// DeleteAssessment removes an assessment permanently.
func (s *AssessmentService) DeleteAssessment(ctx context.Context, id string) error {
	return s.repo.Delete(ctx, id)
}

// ─────────────────────────────────────────────
// Private helpers
// ─────────────────────────────────────────────

func loadQuestionBank(path string) (*models.QuestionBank, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %q: %w", path, err)
	}
	defer f.Close()

	var bank models.QuestionBank
	if err := json.NewDecoder(f).Decode(&bank); err != nil {
		return nil, fmt.Errorf("decode question bank: %w", err)
	}
	if len(bank.Questions) == 0 {
		return nil, fmt.Errorf("question bank is empty")
	}
	return &bank, nil
}

func (s *AssessmentService) validQuestionIDs() map[string]struct{} {
	m := make(map[string]struct{}, len(s.bank.Questions))
	for _, q := range s.bank.Questions {
		m[q.ID] = struct{}{}
	}
	return m
}
