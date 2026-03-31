// internal/validator/validator_test.go
package validator

import (
	"strings"
	"testing"

	"github.com/yourorg/ai-readiness-backend/internal/models"
)

func ptr(i int) *int { return &i }

func validIDs(ids ...string) map[string]struct{} {
	m := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		m[id] = struct{}{}
	}
	return m
}

// ─────────────────────────────────────────────
// ValidateAnswers
// ─────────────────────────────────────────────

func TestValidateAnswers_Valid(t *testing.T) {
	answers := map[string]models.Answer{
		"s1": {Score: ptr(3), Comment: "notes"},
		"s2": {Score: ptr(5)},
		"t1": {Score: ptr(1)},
	}
	if err := ValidateAnswers(answers, validIDs("s1", "s2", "t1")); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestValidateAnswers_EmptyMap(t *testing.T) {
	err := ValidateAnswers(map[string]models.Answer{}, validIDs("s1"))
	if err == nil {
		t.Fatal("expected error for empty answers map")
	}
	if !IsValidationError(err) {
		t.Error("expected ValidationError")
	}
}

func TestValidateAnswers_ScoreZero(t *testing.T) {
	err := ValidateAnswers(
		map[string]models.Answer{"s1": {Score: ptr(0)}},
		validIDs("s1"),
	)
	if err == nil {
		t.Fatal("expected error for score=0")
	}
	ve := err.(*ValidationError)
	if len(ve.Errors) == 0 {
		t.Error("expected at least one field error")
	}
	if !strings.Contains(ve.Errors[0].Field, "s1") {
		t.Errorf("expected error to reference s1, got: %s", ve.Errors[0].Field)
	}
}

func TestValidateAnswers_ScoreSix(t *testing.T) {
	err := ValidateAnswers(
		map[string]models.Answer{"s1": {Score: ptr(6)}},
		validIDs("s1"),
	)
	if err == nil {
		t.Fatal("expected error for score=6")
	}
}

func TestValidateAnswers_AllValidScores(t *testing.T) {
	ids := validIDs("q1", "q2", "q3", "q4", "q5")
	for score := 1; score <= 5; score++ {
		qid := "q" + string(rune('0'+score))
		err := ValidateAnswers(
			map[string]models.Answer{qid: {Score: ptr(score)}},
			ids,
		)
		if err != nil {
			t.Errorf("score %d should be valid, got: %v", score, err)
		}
	}
}

func TestValidateAnswers_UnknownQuestionID(t *testing.T) {
	err := ValidateAnswers(
		map[string]models.Answer{"unknown_xyz": {Score: ptr(3)}},
		validIDs("s1", "s2"),
	)
	if err == nil {
		t.Fatal("expected error for unknown question ID")
	}
	ve := err.(*ValidationError)
	found := false
	for _, fe := range ve.Errors {
		if strings.Contains(fe.Message, "unknown_xyz") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected error mentioning unknown_xyz, got: %v", ve.Errors)
	}
}

func TestValidateAnswers_CommentTooLong(t *testing.T) {
	longComment := strings.Repeat("x", 2001)
	err := ValidateAnswers(
		map[string]models.Answer{"s1": {Score: ptr(3), Comment: longComment}},
		validIDs("s1"),
	)
	if err == nil {
		t.Fatal("expected error for comment > 2000 chars")
	}
}

func TestValidateAnswers_CommentExactlyLimit(t *testing.T) {
	comment := strings.Repeat("x", 2000)
	err := ValidateAnswers(
		map[string]models.Answer{"s1": {Score: ptr(3), Comment: comment}},
		validIDs("s1"),
	)
	if err != nil {
		t.Errorf("2000-char comment should be valid, got: %v", err)
	}
}

func TestValidateAnswers_MultipleErrors(t *testing.T) {
	err := ValidateAnswers(
		map[string]models.Answer{
			"s1": {Score: ptr(6)},           // bad score
			"unknown": {Score: ptr(3)},       // unknown ID
		},
		validIDs("s1"),
	)
	if err == nil {
		t.Fatal("expected multiple validation errors")
	}
	ve := err.(*ValidationError)
	if len(ve.Errors) < 2 {
		t.Errorf("expected ≥2 errors, got %d: %v", len(ve.Errors), ve.Errors)
	}
}

func TestValidateAnswers_NilScore_IsAllowed(t *testing.T) {
	// A nil score means "not yet answered" — should be allowed in a merge batch
	err := ValidateAnswers(
		map[string]models.Answer{"s1": {Comment: "will answer later"}},
		validIDs("s1"),
	)
	if err != nil {
		t.Errorf("nil score with comment should be valid, got: %v", err)
	}
}

// ─────────────────────────────────────────────
// ValidatePagination
// ─────────────────────────────────────────────

func TestValidatePagination_Defaults(t *testing.T) {
	limit, offset, err := ValidatePagination(0, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 20 {
		t.Errorf("expected default limit 20, got %d", limit)
	}
	if offset != 0 {
		t.Errorf("expected offset 0, got %d", offset)
	}
}

func TestValidatePagination_LimitOver100(t *testing.T) {
	_, _, err := ValidatePagination(101, 0)
	if err == nil {
		t.Error("expected error for limit > 100")
	}
}

func TestValidatePagination_NegativeOffset(t *testing.T) {
	_, _, err := ValidatePagination(10, -1)
	if err == nil {
		t.Error("expected error for negative offset")
	}
}

func TestValidatePagination_Valid(t *testing.T) {
	limit, offset, err := ValidatePagination(50, 100)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if limit != 50 || offset != 100 {
		t.Errorf("got limit=%d offset=%d", limit, offset)
	}
}

// ─────────────────────────────────────────────
// ValidateObjectID
// ─────────────────────────────────────────────

func TestValidateObjectID_Valid(t *testing.T) {
	cases := []string{
		"6643f1a2c3d4e5f6a7b8c9d0",
		"000000000000000000000000",
		"ffffffffffffffffffffffff",
	}
	for _, id := range cases {
		if err := ValidateObjectID(id); err != nil {
			t.Errorf("valid ID %q should pass, got: %v", id, err)
		}
	}
}

func TestValidateObjectID_TooShort(t *testing.T) {
	if err := ValidateObjectID("abc123"); err == nil {
		t.Error("expected error for short ID")
	}
}

func TestValidateObjectID_TooLong(t *testing.T) {
	if err := ValidateObjectID("6643f1a2c3d4e5f6a7b8c9d0aa"); err == nil {
		t.Error("expected error for long ID")
	}
}

func TestValidateObjectID_InvalidChars(t *testing.T) {
	if err := ValidateObjectID("6643f1a2c3d4e5f6a7b8ZZZZ"); err == nil {
		t.Error("expected error for non-hex characters")
	}
}

func TestValidateObjectID_Empty(t *testing.T) {
	if err := ValidateObjectID(""); err == nil {
		t.Error("expected error for empty string")
	}
}

// ─────────────────────────────────────────────
// ValidationError
// ─────────────────────────────────────────────

func TestValidationError_ErrorString(t *testing.T) {
	ve := &ValidationError{Errors: []FieldError{
		{Field: "score", Message: "must be 1-5"},
		{Field: "comment", Message: "too long"},
	}}
	s := ve.Error()
	if !strings.Contains(s, "score") || !strings.Contains(s, "comment") {
		t.Errorf("error string missing fields: %q", s)
	}
}

func TestIsValidationError(t *testing.T) {
	ve := &ValidationError{}
	if !IsValidationError(ve) {
		t.Error("expected IsValidationError=true for *ValidationError")
	}
	if IsValidationError(nil) {
		t.Error("expected IsValidationError=false for nil")
	}
}
