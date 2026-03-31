// internal/scoring/engine_extended_test.go
// Additional scoring tests focusing on edge cases, helper functions,
// recommendation generation, and domain weight correctness.
package scoring

import (
	"math"
	"testing"

	"github.com/yourorg/ai-readiness-backend/internal/models"
)

// ─────────────────────────────────────────────
// roundTwo
// ─────────────────────────────────────────────

func TestRoundTwo(t *testing.T) {
	cases := []struct{ in, want float64 }{
		{0.0, 0.0},
		{1.0, 1.0},
		{1.005, 1.01},
		{1.004, 1.0},
		{63.425, 63.43},
		{100.0, 100.0},
		{99.999, 100.0},
	}
	for _, tc := range cases {
		got := roundTwo(tc.in)
		if math.Abs(got-tc.want) > 0.001 {
			t.Errorf("roundTwo(%.4f) = %.4f, want %.4f", tc.in, got, tc.want)
		}
	}
}

// ─────────────────────────────────────────────
// scoreToPriority / scoreToPhase
// ─────────────────────────────────────────────

func TestScoreToPriority(t *testing.T) {
	cases := []struct {
		score    float64
		expected string
	}{
		{0, "Critical"},
		{39.9, "Critical"},
		{40, "High"},
		{64.9, "High"},
		{65, "Medium"},
		{100, "Medium"},
	}
	for _, tc := range cases {
		if got := scoreToPriority(tc.score); got != tc.expected {
			t.Errorf("scoreToPriority(%.1f) = %q, want %q", tc.score, got, tc.expected)
		}
	}
}

func TestScoreToPhase(t *testing.T) {
	cases := []struct {
		score    float64
		expected string
	}{
		{0, "Phase 1"},
		{39.9, "Phase 1"},
		{40, "Phase 2"},
		{64.9, "Phase 2"},
		{65, "Phase 3"},
		{100, "Phase 3"},
	}
	for _, tc := range cases {
		if got := scoreToPhase(tc.score); got != tc.expected {
			t.Errorf("scoreToPhase(%.1f) = %q, want %q", tc.score, got, tc.expected)
		}
	}
}

// ─────────────────────────────────────────────
// phaseOrder / priorityOrder
// ─────────────────────────────────────────────

func TestPhaseOrder_Sorted(t *testing.T) {
	if phaseOrder("Phase 1") >= phaseOrder("Phase 2") {
		t.Error("Phase 1 should sort before Phase 2")
	}
	if phaseOrder("Phase 2") >= phaseOrder("Phase 3") {
		t.Error("Phase 2 should sort before Phase 3")
	}
}

func TestPriorityOrder_Sorted(t *testing.T) {
	if priorityOrder("Critical") >= priorityOrder("High") {
		t.Error("Critical should sort before High")
	}
	if priorityOrder("High") >= priorityOrder("Medium") {
		t.Error("High should sort before Medium")
	}
}

// ─────────────────────────────────────────────
// Domain weight correctness
// ─────────────────────────────────────────────

func TestDomainWeights_SumToOne(t *testing.T) {
	total := 0.0
	for _, w := range DomainWeights {
		total += w
	}
	if math.Abs(total-1.0) > 0.0001 {
		t.Errorf("domain weights should sum to 1.0, got %.4f", total)
	}
}

func TestDomainWeights_AllDomainsPresent(t *testing.T) {
	required := []string{"Strategic", "Technology", "Data", "Organization", "Security", "UseCase"}
	for _, d := range required {
		if _, ok := DomainWeights[d]; !ok {
			t.Errorf("missing domain weight for %q", d)
		}
	}
}

func TestDomainWeights_AllPositive(t *testing.T) {
	for domain, w := range DomainWeights {
		if w <= 0 {
			t.Errorf("domain %q has non-positive weight %.4f", domain, w)
		}
	}
}

// ─────────────────────────────────────────────
// Recommendation generation
// ─────────────────────────────────────────────

func TestRecommendations_Phase1ForLowScores(t *testing.T) {
	bank := makeBank()
	answers := allScores(bank, 1) // all 1s → all domains near 0

	r := Compute(bank, answers)

	for _, rec := range r.Recommendations {
		if rec.Phase != "Phase 1" {
			t.Errorf("all-low scores: expected Phase 1 recommendation, got %q (%s)", rec.Phase, rec.Domain)
		}
	}
}

func TestRecommendations_Phase3ForHighScores(t *testing.T) {
	bank := makeBank()
	answers := allScores(bank, 5) // all 5s → all domains at 100

	r := Compute(bank, answers)

	for _, rec := range r.Recommendations {
		if rec.Phase != "Phase 3" {
			t.Errorf("all-high scores: expected Phase 3 recommendation, got %q (%s)", rec.Phase, rec.Domain)
		}
	}
}

func TestRecommendations_MaxFifteen(t *testing.T) {
	bank := makeBank()
	answers := allScores(bank, 1) // maximise recommendations

	r := Compute(bank, answers)

	if len(r.Recommendations) > 15 {
		t.Errorf("expected ≤15 recommendations, got %d", len(r.Recommendations))
	}
}

func TestRecommendations_SortedPhaseFirst(t *testing.T) {
	bank := makeFullBank()
	// Mixed scores to get all three phases
	answers := map[string]models.Answer{}
	for i, q := range bank.Questions {
		score := (i%5 + 1)
		answers[q.ID] = models.Answer{Score: &score}
	}

	r := Compute(bank, answers)

	phaseOrder := map[string]int{"Phase 1": 0, "Phase 2": 1, "Phase 3": 2}
	for i := 1; i < len(r.Recommendations); i++ {
		prev := r.Recommendations[i-1]
		curr := r.Recommendations[i]
		if phaseOrder[prev.Phase] > phaseOrder[curr.Phase] {
			t.Errorf("recommendations not sorted by phase: %s before %s", curr.Phase, prev.Phase)
		}
	}
}

func TestRecommendations_AllHaveRequiredFields(t *testing.T) {
	bank := makeBank()
	answers := allScores(bank, 2)
	r := Compute(bank, answers)

	for i, rec := range r.Recommendations {
		if rec.Domain == "" {
			t.Errorf("rec[%d]: missing domain", i)
		}
		if rec.Text == "" {
			t.Errorf("rec[%d]: missing text", i)
		}
		if rec.Priority == "" {
			t.Errorf("rec[%d]: missing priority", i)
		}
		if rec.Phase == "" {
			t.Errorf("rec[%d]: missing phase", i)
		}
		validPriority := rec.Priority == "Critical" || rec.Priority == "High" || rec.Priority == "Medium"
		if !validPriority {
			t.Errorf("rec[%d]: invalid priority %q", i, rec.Priority)
		}
		validPhase := rec.Phase == "Phase 1" || rec.Phase == "Phase 2" || rec.Phase == "Phase 3"
		if !validPhase {
			t.Errorf("rec[%d]: invalid phase %q", i, rec.Phase)
		}
	}
}

// ─────────────────────────────────────────────
// Overall score edge cases
// ─────────────────────────────────────────────

func TestCompute_NoAnswers_OverallZero(t *testing.T) {
	bank := makeBank()
	r := Compute(bank, map[string]models.Answer{})
	if r.Overall != 0 {
		t.Errorf("no answers: expected overall=0, got %.4f", r.Overall)
	}
	if r.Confidence != 0 {
		t.Errorf("no answers: expected confidence=0, got %.4f", r.Confidence)
	}
}

func TestCompute_SingleDomainAnswered_PartialResult(t *testing.T) {
	bank := makeBank()
	answers := map[string]models.Answer{}
	// Only answer Strategic domain
	for _, q := range bank.Questions {
		if q.Domain == "Strategic" {
			s := 5
			answers[q.ID] = models.Answer{Score: &s}
		}
	}
	r := Compute(bank, answers)

	// Strategic at 100 × 0.20 weight = 20.0, all others 0
	if r.Overall <= 0 {
		t.Errorf("expected non-zero overall with Strategic answered, got %.4f", r.Overall)
	}
	// Expect roughly 20 (Strategic weight × 100)
	if math.Abs(r.Overall-20.0) > 1.0 {
		t.Errorf("expected overall ≈20.0 with only Strategic at 100, got %.4f", r.Overall)
	}
}

func TestCompute_Score3_GivesExactlyFifty(t *testing.T) {
	// score 3 normalized = ((3-1)/4)*100 = 50.0 exactly
	bank := makeBank()
	answers := allScores(bank, 3)
	r := Compute(bank, answers)

	if math.Abs(r.Overall-50.0) > 0.01 {
		t.Errorf("all-3 answers: expected overall=50.0, got %.4f", r.Overall)
	}
}

// ─────────────────────────────────────────────
// Security-specific risk cap interaction
// ─────────────────────────────────────────────

func TestCompute_SecurityLow_DoesNotCapLowOverall(t *testing.T) {
	// If overall < 75, maturity cap (MATURITY_CAPPED) should NOT trigger
	// even if Security < 50
	bank := makeBank()
	answers := allScores(bank, 2) // overall ~= 25
	for _, q := range bank.Questions {
		if q.Domain == "Security" {
			answers[q.ID] = models.Answer{Score: intPtr(1)}
		}
	}
	r := Compute(bank, answers)

	for _, risk := range r.Risks {
		if risk == "MATURITY_CAPPED" {
			t.Error("MATURITY_CAPPED should not trigger when overall < 75")
		}
	}
}

func TestCompute_BothDataAndSecurity_Low_BothRiskFlags(t *testing.T) {
	bank := makeFullBank()
	answers := allFullScores(bank, 4) // start high

	// Drive Data and Security to 0
	for _, q := range bank.Questions {
		if q.Domain == "Data" || q.Domain == "Security" {
			answers[q.ID] = models.Answer{Score: intPtr(1)}
		}
	}
	r := Compute(bank, answers)

	hasData := false
	hasSec := false
	for _, risk := range r.Risks {
		if risk == "DATA_HIGH_RISK" {
			hasData = true
		}
		if risk == "SECURITY_HIGH_RISK" {
			hasSec = true
		}
	}
	if !hasData {
		t.Error("expected DATA_HIGH_RISK")
	}
	if !hasSec {
		t.Error("expected SECURITY_HIGH_RISK")
	}
}

// ─────────────────────────────────────────────
// Test bank builders
// ─────────────────────────────────────────────

func makeFullBank() *models.QuestionBank {
	domains := []string{"Strategic", "Technology", "Data", "Organization", "Security", "UseCase"}
	qids := map[string][]string{
		"Strategic":    {"s1", "s2", "s3", "s4", "s5", "s6", "s7", "s8", "s9", "s10", "s11", "s12"},
		"Technology":   {"t1", "t2", "t3", "t4", "t5", "t6", "t7", "t8", "t9", "t10", "t11", "t12"},
		"Data":         {"d1", "d2", "d3", "d4", "d5", "d6", "d7", "d8", "d9", "d10", "d11", "d12"},
		"Organization": {"o1", "o2", "o3", "o4", "o5", "o6", "o7", "o8", "o9", "o10", "o11", "o12"},
		"Security":     {"sec1", "sec2", "sec3", "sec4", "sec5", "sec6", "sec7", "sec8", "sec9", "sec10", "sec11", "sec12"},
		"UseCase":      {"u1", "u2", "u3", "u4", "u5", "u6", "u7", "u8", "u9", "u10", "u11", "u12"},
	}
	weights := []int{3, 3, 2, 2, 2, 2, 1, 2, 1, 1, 1, 1}
	criticals := []bool{true, true, false, false, false, false, false, false, false, false, false, false}

	var questions []models.Question
	for _, d := range domains {
		for i, qid := range qids[d] {
			questions = append(questions, models.Question{
				ID: qid, Domain: d, Weight: weights[i], IsCritical: criticals[i],
			})
		}
	}
	return &models.QuestionBank{Domains: domains, Questions: questions}
}

func allFullScores(bank *models.QuestionBank, score int) map[string]models.Answer {
	m := make(map[string]models.Answer, len(bank.Questions))
	for _, q := range bank.Questions {
		s := score
		m[q.ID] = models.Answer{Score: &s}
	}
	return m
}

func intPtr(i int) *int { return &i }
