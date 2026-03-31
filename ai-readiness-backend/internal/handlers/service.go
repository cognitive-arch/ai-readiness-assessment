// internal/handlers/service.go
package handlers

import (
	"context"

	"github.com/yourorg/ai-readiness-backend/internal/models"
)

// AssessmentServicer is the interface the handler depends on.
// Using an interface here keeps handlers testable without a real MongoDB connection.
type AssessmentServicer interface {
	CreateAssessment(ctx context.Context, clientRef string) (*models.Assessment, error)
	GetAssessment(ctx context.Context, id string) (*models.Assessment, error)
	SaveAnswers(ctx context.Context, id string, answers map[string]models.Answer) (*models.Assessment, error)
	ComputeResult(ctx context.Context, id string) (*models.Assessment, error)
	GetResult(ctx context.Context, id string) (*models.Result, error)
	ListAssessments(ctx context.Context, limit, offset int64) ([]*models.Assessment, int64, error)
	DeleteAssessment(ctx context.Context, id string) error
	QuestionBank() *models.QuestionBank
}
