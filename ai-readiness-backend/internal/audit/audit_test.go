// internal/audit/audit_test.go
package audit

import (
	"context"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func newObservedAudit(t *testing.T) (*Logger, *observer.ObservedLogs) {
	t.Helper()
	core, logs := observer.New(zap.InfoLevel)
	log := zap.New(core)
	return New(log), logs
}

func TestAssessmentCreated_LogsCorrectFields(t *testing.T) {
	a, logs := newObservedAudit(t)
	ctx := context.Background()

	a.AssessmentCreated(ctx, "abc123", "acme-corp", "192.168.1.1")

	if logs.Len() != 1 {
		t.Fatalf("expected 1 log entry, got %d", logs.Len())
	}
	entry := logs.All()[0]

	assertField(t, entry, "event_type", string(EventAssessmentCreated))
	assertField(t, entry, "assessment_id", "abc123")
	assertField(t, entry, "client_ref", "acme-corp")
	assertField(t, entry, "remote_addr", "192.168.1.1")
}

func TestAnswersSaved_LogsCount(t *testing.T) {
	a, logs := newObservedAudit(t)
	a.AnswersSaved(context.Background(), "def456", 12)

	entry := logs.All()[0]
	assertField(t, entry, "event_type", string(EventAnswersSaved))
	assertField(t, entry, "assessment_id", "def456")
	assertField(t, entry, "meta.answer_count", "12")
}

func TestAssessmentComputed_LogsMaturityAndRisks(t *testing.T) {
	a, logs := newObservedAudit(t)
	a.AssessmentComputed(context.Background(), "ghi789", "AI Structured", 63.42,
		[]string{"CRITICAL_GAPS", "DATA_HIGH_RISK"})

	entry := logs.All()[0]
	assertField(t, entry, "meta.maturity", "AI Structured")
	assertField(t, entry, "meta.risk.CRITICAL_GAPS", "true")
	assertField(t, entry, "meta.risk.DATA_HIGH_RISK", "true")
}

func TestAssessmentDeleted_LogsRemoteAddr(t *testing.T) {
	a, logs := newObservedAudit(t)
	a.AssessmentDeleted(context.Background(), "jkl012", "10.0.0.1")

	entry := logs.All()[0]
	assertField(t, entry, "event_type", string(EventAssessmentDeleted))
	assertField(t, entry, "remote_addr", "10.0.0.1")
}

func TestPDFExported_Success(t *testing.T) {
	a, logs := newObservedAudit(t)
	a.PDFExported(context.Background(), "mno345", true)

	entry := logs.All()[0]
	assertField(t, entry, "meta.outcome", "success")
}

func TestPDFExported_Failure(t *testing.T) {
	a, logs := newObservedAudit(t)
	a.PDFExported(context.Background(), "pqr678", false)

	entry := logs.All()[0]
	assertField(t, entry, "meta.outcome", "failure")
}

func TestWithRequestID_PropagatedToLog(t *testing.T) {
	a, logs := newObservedAudit(t)
	ctx := WithRequestID(context.Background(), "req-uuid-9999")
	a.ResultFetched(ctx, "stu901")

	entry := logs.All()[0]
	assertField(t, entry, "request_id", "req-uuid-9999")
}

func TestLog_OccurredAt_IsSet(t *testing.T) {
	a, logs := newObservedAudit(t)
	a.AssessmentCreated(context.Background(), "vwx234", "", "")

	entry := logs.All()[0]
	found := false
	for _, f := range entry.Context {
		if f.Key == "occurred_at" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected occurred_at field in audit log entry")
	}
}

func TestItoa(t *testing.T) {
	cases := []struct{ in int; want string }{{0,"0"},{1,"1"},{12,"12"},{100,"100"},{999,"999"}}
	for _, c := range cases {
		if got := itoa(c.in); got != c.want {
			t.Errorf("itoa(%d) = %q, want %q", c.in, got, c.want)
		}
	}
}

func assertField(t *testing.T, entry observer.LoggedEntry, key, want string) {
	t.Helper()
	for _, f := range entry.Context {
		if f.Key == key {
			if got := f.String; got != want {
				t.Errorf("field %q: got %q, want %q", key, got, want)
			}
			return
		}
	}
	t.Errorf("field %q not found in log entry", key)
}
