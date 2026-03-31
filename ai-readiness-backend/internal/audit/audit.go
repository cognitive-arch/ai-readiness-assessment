// internal/audit/audit.go
// Structured audit trail for all state-changing assessment operations.
// Events are written as structured JSON log entries via zap, making them
// easily shipped to any log aggregator (Loki, Datadog, CloudWatch, etc.).
//
// Event types:
//
//	assessment.created    — new assessment document created
//	assessment.answers_saved — answers batch merged
//	assessment.computed   — scoring engine ran, result stored
//	assessment.deleted    — assessment permanently removed
//	pdf.exported          — PDF report generated
package audit

import (
	"context"
	"time"

	"go.uber.org/zap"
)

// EventType is a string enum for audit event kinds.
type EventType string

const (
	EventAssessmentCreated  EventType = "assessment.created"
	EventAnswersSaved       EventType = "assessment.answers_saved"
	EventAssessmentComputed EventType = "assessment.computed"
	EventAssessmentDeleted  EventType = "assessment.deleted"
	EventPDFExported        EventType = "pdf.exported"
	EventResultFetched      EventType = "assessment.result_fetched"
)

// Event is the structured payload written for every audit record.
type Event struct {
	Type         EventType         `json:"type"`
	AssessmentID string            `json:"assessment_id,omitempty"`
	ClientRef    string            `json:"client_ref,omitempty"`
	RequestID    string            `json:"request_id,omitempty"`
	RemoteAddr   string            `json:"remote_addr,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	OccurredAt   time.Time         `json:"occurred_at"`
}

// contextKey is an unexported type for context keys to avoid collisions.
type contextKey int

const requestIDKey contextKey = 0

// Logger writes audit events using a dedicated zap logger.
type Logger struct {
	log *zap.Logger
}

// New returns an audit Logger wrapping the provided zap logger.
// In production, configure the zap logger to ship to your log aggregator.
func New(log *zap.Logger) *Logger {
	return &Logger{log: log.Named("audit")}
}

// Log emits an audit event. Non-blocking.
func (a *Logger) Log(ctx context.Context, event Event) {
	event.OccurredAt = time.Now().UTC()
	event.RequestID = requestIDFromContext(ctx)

	fields := []zap.Field{
		zap.String("event_type", string(event.Type)),
		zap.Time("occurred_at", event.OccurredAt),
	}
	if event.AssessmentID != "" {
		fields = append(fields, zap.String("assessment_id", event.AssessmentID))
	}
	if event.ClientRef != "" {
		fields = append(fields, zap.String("client_ref", event.ClientRef))
	}
	if event.RequestID != "" {
		fields = append(fields, zap.String("request_id", event.RequestID))
	}
	if event.RemoteAddr != "" {
		fields = append(fields, zap.String("remote_addr", event.RemoteAddr))
	}
	for k, v := range event.Metadata {
		fields = append(fields, zap.String("meta."+k, v))
	}

	a.log.Info("audit", fields...)
}

// Convenience constructors
func (a *Logger) AssessmentCreated(ctx context.Context, id, clientRef, remoteAddr string) {
	a.Log(ctx, Event{
		Type:         EventAssessmentCreated,
		AssessmentID: id,
		ClientRef:    clientRef,
		RemoteAddr:   remoteAddr,
	})
}

func (a *Logger) AnswersSaved(ctx context.Context, id string, count int) {
	a.Log(ctx, Event{
		Type:         EventAnswersSaved,
		AssessmentID: id,
		Metadata:     map[string]string{"answer_count": itoa(count)},
	})
}

func (a *Logger) AssessmentComputed(ctx context.Context, id, maturity string, overall float64, risks []string) {
	meta := map[string]string{
		"maturity": maturity,
		"overall":  ftoa(overall),
	}
	for _, r := range risks {
		meta["risk."+r] = "true"
	}
	a.Log(ctx, Event{
		Type:         EventAssessmentComputed,
		AssessmentID: id,
		Metadata:     meta,
	})
}

func (a *Logger) AssessmentDeleted(ctx context.Context, id, remoteAddr string) {
	a.Log(ctx, Event{
		Type:         EventAssessmentDeleted,
		AssessmentID: id,
		RemoteAddr:   remoteAddr,
	})
}

func (a *Logger) PDFExported(ctx context.Context, id string, success bool) {
	outcome := "success"
	if !success {
		outcome = "failure"
	}
	a.Log(ctx, Event{
		Type:         EventPDFExported,
		AssessmentID: id,
		Metadata:     map[string]string{"outcome": outcome},
	})
}

func (a *Logger) ResultFetched(ctx context.Context, id string) {
	a.Log(ctx, Event{
		Type:         EventResultFetched,
		AssessmentID: id,
	})
}

// Context helpers

// WithRequestID stores a request ID in the context for audit log enrichment.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func requestIDFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(requestIDKey).(string); ok {
		return v
	}
	return ""
}

// Mini formatting helpers (no fmt import overhead)
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	buf := [20]byte{}
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}

func ftoa(f float64) string {
	// Simple fixed-2 decimal for audit logs — avoids importing strconv/fmt
	i := int64(f * 100)
	s := itoa(int(i))
	if len(s) <= 2 {
		for len(s) < 3 {
			s = "0" + s
		}
	}
	return s[:len(s)-2] + "." + s[len(s)-2:]
}
