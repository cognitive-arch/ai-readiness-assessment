// internal/handlers/dto.go
package handlers

import "github.com/yourorg/ai-readiness-backend/internal/models"

// ─────────────────────────────────────────────
// Request DTOs
// ─────────────────────────────────────────────

// CreateAssessmentRequest is the body for POST /api/assessment.
type CreateAssessmentRequest struct {
	ClientRef string `json:"client_ref"` // optional
}

// SaveAnswersRequest is the body for PUT /api/assessment/{id}/answers.
// Answers map: questionID -> {score, comment}
type SaveAnswersRequest struct {
	Answers map[string]models.Answer `json:"answers"`
}

// ─────────────────────────────────────────────
// Response envelope
// ─────────────────────────────────────────────

type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Meta    interface{} `json:"meta,omitempty"`
}

type paginationMeta struct {
	Total  int64 `json:"total"`
	Limit  int64 `json:"limit"`
	Offset int64 `json:"offset"`
}

// ─────────────────────────────────────────────
// Response DTOs
// ─────────────────────────────────────────────

// AssessmentResponse is the external shape of an assessment. The ID is exposed
// as a string (hex ObjectID) for easy frontend use.
type AssessmentResponse struct {
	ID        string                    `json:"assessmentId"`
	Status    string                    `json:"status"`
	Answers   map[string]models.Answer  `json:"answers"`
	Result    *models.Result            `json:"result,omitempty"`
	CreatedAt string                    `json:"createdAt"`
	UpdatedAt string                    `json:"updatedAt"`
	ClientRef string                    `json:"clientRef,omitempty"`
}
