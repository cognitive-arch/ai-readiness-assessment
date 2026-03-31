// internal/models/models.go

package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Assessment status enum

type AssessmentStatus string

const (
	StatusDraft      AssessmentStatus = "draft"
	StatusInProgress AssessmentStatus = "in_progress"
	StatusCompleted  AssessmentStatus = "completed"
	StatusComputed   AssessmentStatus = "computed"
)

// Answer — a single question response

type Answer struct {
	Score   *int   `bson:"score,omitempty"   json:"score,omitempty"`
	Comment string `bson:"comment,omitempty" json:"comment,omitempty"`
}

// Assessment — the root document stored in MongoDB

type Assessment struct {
	ID        primitive.ObjectID `bson:"_id,omitempty"     json:"id"`
	Status    AssessmentStatus   `bson:"status"            json:"status"`
	Answers   map[string]Answer  `bson:"answers"           json:"answers"`
	Result    *Result            `bson:"result,omitempty"  json:"result,omitempty"`
	CreatedAt time.Time          `bson:"created_at"        json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at"        json:"updated_at"`
	// Optional metadata for multi-tenant or user-linked workflows
	ClientRef string `bson:"client_ref,omitempty" json:"client_ref,omitempty"`
}

// Scoring result — embedded in Assessment after /compute

type DomainScore struct {
	Score    float64 `bson:"score"    json:"score"`
	Answered int     `bson:"answered" json:"answered"`
	Total    int     `bson:"total"    json:"total"`
}

type Recommendation struct {
	Domain   string `bson:"domain"    json:"domain"`
	Text     string `bson:"text"      json:"text"`
	Priority string `bson:"priority"  json:"priority"` // Critical | High | Medium
	Phase    string `bson:"phase"     json:"phase"`    // Phase 1 | Phase 2 | Phase 3
}

type Result struct {
	Overall         float64                `bson:"overall"          json:"overall"`
	DomainScores    map[string]DomainScore `bson:"domain_scores"    json:"domainScores"`
	Maturity        string                 `bson:"maturity"         json:"maturity"`
	Confidence      float64                `bson:"confidence"       json:"confidence"`
	Risks           []string               `bson:"risks"            json:"risks"`
	Recommendations []Recommendation       `bson:"recommendations"  json:"recommendations"`
	TotalAnswered   int                    `bson:"total_answered"   json:"totalAnswered"`
	TotalQ          int                    `bson:"total_q"          json:"totalQ"`
	ComputedAt      time.Time              `bson:"computed_at"      json:"computedAt"`
}

// Question bank types (in-memory, loaded once)

type Question struct {
	ID         string `json:"id"`
	Domain     string `json:"domain"`
	Text       string `json:"text"`
	Weight     int    `json:"weight"`
	IsCritical bool   `json:"isCritical"`
	Impact     int    `json:"impact"`
}

type QuestionBank struct {
	Domains   []string   `json:"domains"`
	Questions []Question `json:"questions"`
}
