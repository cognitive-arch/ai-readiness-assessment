// internal/scoring/engine_test.go
package scoring

import (
	"testing"

	"github.com/yourorg/ai-readiness-backend/internal/models"
)

func ptr(i int) *int { return &i }

// makeBank builds a minimal question bank for testing.
func makeBank() *models.QuestionBank {
	return &models.QuestionBank{
		Domains: []string{"Strategic", "Technology", "Data", "Organization", "Security", "UseCase"},
		Questions: []models.Question{
			// Strategic — 2 questions: one critical
			{ID: "s1", Domain: "Strategic", Weight: 3, IsCritical: true},
			{ID: "s2", Domain: "Strategic", Weight: 2, IsCritical: false},
			// Technology
			{ID: "t1", Domain: "Technology", Weight: 3, IsCritical: true},
			{ID: "t2", Domain: "Technology", Weight: 2, IsCritical: false},
			// Data
			{ID: "d1", Domain: "Data", Weight: 3, IsCritical: true},
			{ID: "d2", Domain: "Data", Weight: 2, IsCritical: false},
			// Organization
			{ID: "o1", Domain: "Organization", Weight: 3, IsCritical: true},
			{ID: "o2", Domain: "Organization", Weight: 2, IsCritical: false},
			// Security
			{ID: "sec1", Domain: "Security", Weight: 3, IsCritical: true},
			{ID: "sec2", Domain: "Security", Weight: 2, IsCritical: false},
			// UseCase
			{ID: "u1", Domain: "UseCase", Weight: 3, IsCritical: false},
			{ID: "u2", Domain: "UseCase", Weight: 2, IsCritical: false},
		},
	}
}

func TestNormalization(t *testing.T) {
	// Score 1 should normalize to 0, score 5 to 100
	bank := makeBank()
	answers := map[string]models.Answer{
		"s1": {Score: ptr(1)},
		"s2": {Score: ptr(1)},
		"t1": {Score: ptr(1)}, "t2": {Score: ptr(1)},
		"d1": {Score: ptr(1)}, "d2": {Score: ptr(1)},
		"o1": {Score: ptr(1)}, "o2": {Score: ptr(1)},
		"sec1": {Score: ptr(1)}, "sec2": {Score: ptr(1)},
		"u1": {Score: ptr(1)}, "u2": {Score: ptr(1)},
	}
	r := Compute(bank, answers)
	if r.Overall != 0 {
		t.Errorf("all-1 scores: expected overall=0, got %.2f", r.Overall)
	}
	if r.Maturity != "Foundational Risk Zone" {
		t.Errorf("expected Foundational Risk Zone, got %s", r.Maturity)
	}

	// All 5s → 100
	for k := range answers {
		answers[k] = models.Answer{Score: ptr(5)}
	}
	r = Compute(bank, answers)
	if r.Overall != 100 {
		t.Errorf("all-5 scores: expected overall=100, got %.2f", r.Overall)
	}
	if r.Maturity != "AI-Native" {
		t.Errorf("expected AI-Native, got %s", r.Maturity)
	}
}

func TestMaturityClassification(t *testing.T) {
	cases := []struct {
		score    float64
		expected string
	}{
		{0, "Foundational Risk Zone"},
		{39.9, "Foundational Risk Zone"},
		{40, "AI Emerging"},
		{59.9, "AI Emerging"},
		{60, "AI Structured"},
		{74.9, "AI Structured"},
		{75, "AI Advanced"},
		{89.9, "AI Advanced"},
		{90, "AI-Native"},
		{100, "AI-Native"},
	}
	for _, tc := range cases {
		got := classifyMaturity(tc.score)
		if got != tc.expected {
			t.Errorf("classifyMaturity(%.1f) = %q, want %q", tc.score, got, tc.expected)
		}
	}
}

func TestCriticalGapsRisk(t *testing.T) {
	bank := makeBank()
	answers := allScores(bank, 4) // all good
	// Set a critical question to score 2
	answers["s1"] = models.Answer{Score: ptr(2)}

	r := Compute(bank, answers)
	if !containsRisk(r.Risks, "CRITICAL_GAPS") {
		t.Error("expected CRITICAL_GAPS risk flag when critical question scored 2")
	}
}

func TestDataHighRisk(t *testing.T) {
	bank := makeBank()
	answers := allScores(bank, 4)
	// Drive Data domain to 0
	answers["d1"] = models.Answer{Score: ptr(1)}
	answers["d2"] = models.Answer{Score: ptr(1)}

	r := Compute(bank, answers)
	if !containsRisk(r.Risks, "DATA_HIGH_RISK") {
		t.Error("expected DATA_HIGH_RISK when Data domain < 50")
	}
}

func TestSecurityHighRisk(t *testing.T) {
	bank := makeBank()
	answers := allScores(bank, 4)
	answers["sec1"] = models.Answer{Score: ptr(1)}
	answers["sec2"] = models.Answer{Score: ptr(1)}

	r := Compute(bank, answers)
	if !containsRisk(r.Risks, "SECURITY_HIGH_RISK") {
		t.Error("expected SECURITY_HIGH_RISK when Security domain < 50")
	}
}

func TestMaturityCappedRisk(t *testing.T) {
	bank := makeBank()
	// Give everything a high score except Data
	answers := allScores(bank, 5)
	answers["d1"] = models.Answer{Score: ptr(1)}
	answers["d2"] = models.Answer{Score: ptr(1)}

	r := Compute(bank, answers)
	if !containsRisk(r.Risks, "MATURITY_CAPPED") {
		t.Error("expected MATURITY_CAPPED when overall>=75 but Data<50")
	}
	if r.Maturity != "AI Structured" {
		t.Errorf("expected maturity to be capped at AI Structured, got %s", r.Maturity)
	}
}

func TestConfidence(t *testing.T) {
	bank := makeBank()
	answers := map[string]models.Answer{
		"s1": {Score: ptr(3)},
		"s2": {Score: ptr(3)},
	} // 2 out of 12 answered

	r := Compute(bank, answers)
	expected := 2.0 / 12.0 * 100
	if r.Confidence != roundTwo(expected) {
		t.Errorf("expected confidence %.2f, got %.2f", expected, r.Confidence)
	}
}

func TestWeightedDomainScore(t *testing.T) {
	// Two questions: weight 3 scoring 5, weight 2 scoring 1
	// normalized(5) = 100, normalized(1) = 0
	// weighted = (100*3 + 0*2) / (3+2) = 60
	bank := &models.QuestionBank{
		Domains: []string{"Strategic"},
		Questions: []models.Question{
			{ID: "q1", Domain: "Strategic", Weight: 3, IsCritical: false},
			{ID: "q2", Domain: "Strategic", Weight: 2, IsCritical: false},
		},
	}
	answers := map[string]models.Answer{
		"q1": {Score: ptr(5)},
		"q2": {Score: ptr(1)},
	}
	scores := computeDomainScores(bank, answers)
	if scores["Strategic"].Score != 60.0 {
		t.Errorf("expected domain score 60, got %.2f", scores["Strategic"].Score)
	}
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func allScores(bank *models.QuestionBank, score int) map[string]models.Answer {
	m := make(map[string]models.Answer)
	for _, q := range bank.Questions {
		m[q.ID] = models.Answer{Score: ptr(score)}
	}
	return m
}

func containsRisk(risks []string, target string) bool {
	for _, r := range risks {
		if r == target {
			return true
		}
	}
	return false
}
