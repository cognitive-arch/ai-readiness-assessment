// tests/bench/scoring_bench_test.go
// Benchmark the scoring engine under realistic conditions.
// Run with: go test -bench=. -benchmem ./tests/bench/...
package bench

import (
	"testing"

	"github.com/yourorg/ai-readiness-backend/internal/models"
	"github.com/yourorg/ai-readiness-backend/internal/scoring"
)

// ─────────────────────────────────────────────
// Fixtures
// ─────────────────────────────────────────────

func fullBank() *models.QuestionBank {
	domains := []string{"Strategic", "Technology", "Data", "Organization", "Security", "UseCase"}
	var questions []models.Question

	domainQIDs := map[string][]string{
		"Strategic":    {"s1","s2","s3","s4","s5","s6","s7","s8","s9","s10","s11","s12"},
		"Technology":   {"t1","t2","t3","t4","t5","t6","t7","t8","t9","t10","t11","t12"},
		"Data":         {"d1","d2","d3","d4","d5","d6","d7","d8","d9","d10","d11","d12"},
		"Organization": {"o1","o2","o3","o4","o5","o6","o7","o8","o9","o10","o11","o12"},
		"Security":     {"sec1","sec2","sec3","sec4","sec5","sec6","sec7","sec8","sec9","sec10","sec11","sec12"},
		"UseCase":      {"u1","u2","u3","u4","u5","u6","u7","u8","u9","u10","u11","u12"},
	}

	weights := []int{3, 3, 2, 2, 2, 2, 1, 2, 1, 1, 1, 1}
	criticals := []bool{true, true, false, false, false, false, false, false, false, false, false, false}

	for _, d := range domains {
		for i, qid := range domainQIDs[d] {
			questions = append(questions, models.Question{
				ID:         qid,
				Domain:     d,
				Weight:     weights[i],
				IsCritical: criticals[i],
				Impact:     5 - i/3,
			})
		}
	}
	return &models.QuestionBank{Domains: domains, Questions: questions}
}

func allAnswers(bank *models.QuestionBank, score int) map[string]models.Answer {
	answers := make(map[string]models.Answer, len(bank.Questions))
	for _, q := range bank.Questions {
		s := score
		answers[q.ID] = models.Answer{Score: &s}
	}
	return answers
}

func mixedAnswers(bank *models.QuestionBank) map[string]models.Answer {
	answers := make(map[string]models.Answer, len(bank.Questions))
	scores := []int{1, 2, 3, 4, 5, 3, 4, 2, 5, 1, 4, 3}
	for i, q := range bank.Questions {
		s := scores[i%len(scores)]
		comment := ""
		if i%3 == 0 {
			comment = "Evidence note for question " + q.ID
		}
		answers[q.ID] = models.Answer{Score: &s, Comment: comment}
	}
	return answers
}

// ─────────────────────────────────────────────
// Benchmarks
// ─────────────────────────────────────────────

// BenchmarkScoring_AllAnswered benchmarks a full 72-question compute.
func BenchmarkScoring_AllAnswered(b *testing.B) {
	bank := fullBank()
	answers := allAnswers(bank, 3)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = scoring.Compute(bank, answers)
	}
}

// BenchmarkScoring_MixedAnswers benchmarks with realistic mixed scores and comments.
func BenchmarkScoring_MixedAnswers(b *testing.B) {
	bank := fullBank()
	answers := mixedAnswers(bank)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = scoring.Compute(bank, answers)
	}
}

// BenchmarkScoring_PartialAnswers benchmarks with only 50% of questions answered.
func BenchmarkScoring_PartialAnswers(b *testing.B) {
	bank := fullBank()
	answers := make(map[string]models.Answer, 36)
	for i, q := range bank.Questions {
		if i%2 == 0 {
			s := 3
			answers[q.ID] = models.Answer{Score: &s}
		}
	}

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = scoring.Compute(bank, answers)
	}
}

// BenchmarkScoring_Parallel benchmarks concurrent scoring (e.g. burst of API requests).
func BenchmarkScoring_Parallel(b *testing.B) {
	bank := fullBank()
	answers := mixedAnswers(bank)

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_ = scoring.Compute(bank, answers)
		}
	})
}

// BenchmarkScoring_NoAnswers benchmarks the zero-answer edge case.
func BenchmarkScoring_NoAnswers(b *testing.B) {
	bank := fullBank()
	answers := make(map[string]models.Answer)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = scoring.Compute(bank, answers)
	}
}

// ─────────────────────────────────────────────
// Scoring correctness smoke tests (run alongside benchmarks)
// ─────────────────────────────────────────────

func TestBench_AllFives_GivesHundred(t *testing.T) {
	bank := fullBank()
	answers := allAnswers(bank, 5)
	result := scoring.Compute(bank, answers)
	if result.Overall != 100.0 {
		t.Errorf("all-5: expected 100, got %.4f", result.Overall)
	}
}

func TestBench_AllOnes_GivesZero(t *testing.T) {
	bank := fullBank()
	answers := allAnswers(bank, 1)
	result := scoring.Compute(bank, answers)
	if result.Overall != 0.0 {
		t.Errorf("all-1: expected 0, got %.4f", result.Overall)
	}
}

func TestBench_ConfidenceIsCorrect(t *testing.T) {
	bank := fullBank()
	// Answer only 36 of 72
	answers := make(map[string]models.Answer, 36)
	for i, q := range bank.Questions {
		if i < 36 {
			s := 3
			answers[q.ID] = models.Answer{Score: &s}
		}
	}
	result := scoring.Compute(bank, answers)
	expected := 50.0
	if result.Confidence != expected {
		t.Errorf("expected confidence %.1f, got %.4f", expected, result.Confidence)
	}
}

func TestBench_ScoringIsDeterministic(t *testing.T) {
	bank := fullBank()
	answers := mixedAnswers(bank)

	r1 := scoring.Compute(bank, answers)
	r2 := scoring.Compute(bank, answers)
	r3 := scoring.Compute(bank, answers)

	if r1.Overall != r2.Overall || r2.Overall != r3.Overall {
		t.Errorf("scoring is not deterministic: %.4f %.4f %.4f", r1.Overall, r2.Overall, r3.Overall)
	}
	if r1.Maturity != r2.Maturity {
		t.Errorf("maturity not deterministic: %q %q", r1.Maturity, r2.Maturity)
	}
}
