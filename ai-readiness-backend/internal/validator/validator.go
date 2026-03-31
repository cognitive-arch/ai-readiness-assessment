// internal/validator/validator.go
// Centralised request validation with structured error collection.
// Validates answer payloads before they reach the service layer.
package validator

import (
	"fmt"
	"strings"

	"github.com/yourorg/ai-readiness-backend/internal/models"
)

// ValidationError holds one or more field-level validation failures.
type ValidationError struct {
	Errors []FieldError `json:"errors"`
}

// FieldError describes a single validation failure.
type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (v *ValidationError) Error() string {
	msgs := make([]string, len(v.Errors))
	for i, e := range v.Errors {
		msgs[i] = fmt.Sprintf("%s: %s", e.Field, e.Message)
	}
	return strings.Join(msgs, "; ")
}

// IsValidationError reports whether err is a *ValidationError.
func IsValidationError(err error) bool {
	_, ok := err.(*ValidationError)
	return ok
}

// ─────────────────────────────────────────────
// Answer validation
// ─────────────────────────────────────────────

// ValidateAnswers checks a batch of answers against the known question bank.
// Returns *ValidationError if any answers are invalid; nil otherwise.
func ValidateAnswers(answers map[string]models.Answer, validIDs map[string]struct{}) error {
	if len(answers) == 0 {
		return &ValidationError{Errors: []FieldError{{
			Field:   "answers",
			Message: "must contain at least one answer",
		}}}
	}

	var errs []FieldError

	for qID, ans := range answers {
		// Unknown question ID
		if _, ok := validIDs[qID]; !ok {
			errs = append(errs, FieldError{
				Field:   fmt.Sprintf("answers.%s", qID),
				Message: fmt.Sprintf("unknown question ID %q", qID),
			})
			continue
		}

		// Score present — must be 1-5
		if ans.Score != nil {
			if *ans.Score < 1 || *ans.Score > 5 {
				errs = append(errs, FieldError{
					Field:   fmt.Sprintf("answers.%s.score", qID),
					Message: fmt.Sprintf("must be between 1 and 5, got %d", *ans.Score),
				})
			}
		}

		// Comment length limit
		if len(ans.Comment) > 2000 {
			errs = append(errs, FieldError{
				Field:   fmt.Sprintf("answers.%s.comment", qID),
				Message: fmt.Sprintf("must be ≤ 2000 characters, got %d", len(ans.Comment)),
			})
		}
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// ValidatePagination clamps limit and offset to safe ranges.
// Returns sanitised (limit, offset).
func ValidatePagination(limit, offset int64) (int64, int64, error) {
	var errs []FieldError

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		errs = append(errs, FieldError{
			Field:   "limit",
			Message: "must be ≤ 100",
		})
	}
	if offset < 0 {
		errs = append(errs, FieldError{
			Field:   "offset",
			Message: "must be ≥ 0",
		})
	}

	if len(errs) > 0 {
		return 0, 0, &ValidationError{Errors: errs}
	}
	return limit, offset, nil
}

// ValidateObjectID checks that an ID string looks like a 24-char hex ObjectID.
func ValidateObjectID(id string) error {
	if len(id) != 24 {
		return &ValidationError{Errors: []FieldError{{
			Field:   "id",
			Message: fmt.Sprintf("must be a 24-character hex string, got %d characters", len(id)),
		}}}
	}
	for _, c := range id {
		if !isHex(c) {
			return &ValidationError{Errors: []FieldError{{
				Field:   "id",
				Message: "must contain only hexadecimal characters (0-9, a-f)",
			}}}
		}
	}
	return nil
}

func isHex(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}
