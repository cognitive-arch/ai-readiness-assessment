// internal/service/pdf_test.go
package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yourorg/ai-readiness-backend/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

func makeAssessment(t *testing.T, overall float64, maturity string) *models.Assessment {
	t.Helper()
	risks := []string{}
	if overall < 60 {
		risks = append(risks, "CRITICAL_GAPS", "DATA_HIGH_RISK")
	}
	return &models.Assessment{
		ID:        primitive.NewObjectID(),
		Status:    models.StatusComputed,
		ClientRef: "test-organization",
		CreatedAt: time.Now().Add(-1 * time.Hour),
		UpdatedAt: time.Now(),
		Answers: map[string]models.Answer{
			"s1": {Score: intPtr(4), Comment: "Strategy documented in Confluence"},
			"t1": {Score: intPtr(3)},
			"d1": {Score: intPtr(2), Comment: "Data governance partially implemented"},
		},
		Result: &models.Result{
			Overall:       overall,
			Maturity:      maturity,
			Confidence:    88.9,
			TotalAnswered: 65,
			TotalQ:        72,
			Risks:         risks,
			ComputedAt:    time.Now(),
			DomainScores: map[string]models.DomainScore{
				"Strategic":    {Score: 75.0, Answered: 10, Total: 12},
				"Technology":   {Score: 60.5, Answered: 12, Total: 12},
				"Data":         {Score: 42.0, Answered: 11, Total: 12},
				"Organization": {Score: 68.3, Answered: 12, Total: 12},
				"Security":     {Score: 55.0, Answered: 12, Total: 12},
				"UseCase":      {Score: 70.0, Answered: 8,  Total: 12},
			},
			Recommendations: []models.Recommendation{
				{Domain: "Data",     Text: "Implement unified data catalog",           Priority: "Critical", Phase: "Phase 1"},
				{Domain: "Security", Text: "Conduct AI threat modeling",               Priority: "High",     Phase: "Phase 1"},
				{Domain: "Technology", Text: "Deploy model monitoring tooling",         Priority: "High",     Phase: "Phase 2"},
				{Domain: "Strategic",  Text: "Establish AI governance committee",       Priority: "Medium",   Phase: "Phase 2"},
				{Domain: "Organization", Text: "Launch AI literacy program",            Priority: "Medium",   Phase: "Phase 3"},
			},
		},
	}
}

func intPtr(i int) *int { return &i }

// ─────────────────────────────────────────────
// Tests
// ─────────────────────────────────────────────

func TestPDFExporter_GeneratesFile(t *testing.T) {
	tmpDir := t.TempDir()
	exp := NewPDFExporter(tmpDir)

	a := makeAssessment(t, 63.42, "AI Structured")
	path, err := exp.Generate(a)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	defer CleanupFile(path)

	// File should exist
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("expected PDF file to exist on disk")
	}
}

func TestPDFExporter_FileHasPDFMagicBytes(t *testing.T) {
	tmpDir := t.TempDir()
	exp := NewPDFExporter(tmpDir)

	a := makeAssessment(t, 63.42, "AI Structured")
	path, err := exp.Generate(a)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	defer CleanupFile(path)

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if len(data) < 4 {
		t.Fatalf("PDF too small: %d bytes", len(data))
	}
	if string(data[:4]) != "%PDF" {
		t.Errorf("expected %%PDF header, got: %q", string(data[:4]))
	}
}

func TestPDFExporter_FileSizeReasonable(t *testing.T) {
	tmpDir := t.TempDir()
	exp := NewPDFExporter(tmpDir)

	a := makeAssessment(t, 63.42, "AI Structured")
	path, err := exp.Generate(a)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	defer CleanupFile(path)

	info, _ := os.Stat(path)
	sizeKB := info.Size() / 1024
	if sizeKB < 5 {
		t.Errorf("PDF too small (%d KB) — likely empty or corrupt", sizeKB)
	}
	if sizeKB > 5000 {
		t.Errorf("PDF unexpectedly large (%d KB)", sizeKB)
	}
	t.Logf("PDF size: %d KB", sizeKB)
}

func TestPDFExporter_FileIsInTmpDir(t *testing.T) {
	tmpDir := t.TempDir()
	exp := NewPDFExporter(tmpDir)

	a := makeAssessment(t, 50.0, "AI Emerging")
	path, err := exp.Generate(a)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	defer CleanupFile(path)

	if !strings.HasPrefix(path, tmpDir) {
		t.Errorf("expected PDF in %q, got %q", tmpDir, path)
	}
}

func TestPDFExporter_FileNameContainsAssessmentID(t *testing.T) {
	tmpDir := t.TempDir()
	exp := NewPDFExporter(tmpDir)

	a := makeAssessment(t, 80.0, "AI Advanced")
	path, err := exp.Generate(a)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	defer CleanupFile(path)

	filename := filepath.Base(path)
	if !strings.Contains(filename, a.ID.Hex()) {
		t.Errorf("expected filename to contain assessment ID %s, got %q", a.ID.Hex(), filename)
	}
}

func TestPDFExporter_NoResultReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	exp := NewPDFExporter(tmpDir)

	a := &models.Assessment{
		ID:     primitive.NewObjectID(),
		Status: models.StatusDraft,
		Result: nil, // no result
	}
	_, err := exp.Generate(a)
	if err == nil {
		t.Error("expected error when generating PDF without a result")
	}
}

func TestPDFExporter_AllMaturityLevels(t *testing.T) {
	tmpDir := t.TempDir()
	exp := NewPDFExporter(tmpDir)

	cases := []struct {
		overall  float64
		maturity string
	}{
		{20.0, "Foundational Risk Zone"},
		{50.0, "AI Emerging"},
		{67.5, "AI Structured"},
		{82.0, "AI Advanced"},
		{95.0, "AI-Native"},
	}

	for _, tc := range cases {
		t.Run(tc.maturity, func(t *testing.T) {
			a := makeAssessment(t, tc.overall, tc.maturity)
			path, err := exp.Generate(a)
			if err != nil {
				t.Fatalf("%s: Generate: %v", tc.maturity, err)
			}
			CleanupFile(path)
		})
	}
}

func TestPDFExporter_WithNoRisks(t *testing.T) {
	tmpDir := t.TempDir()
	exp := NewPDFExporter(tmpDir)

	a := makeAssessment(t, 92.0, "AI-Native")
	a.Result.Risks = []string{} // no risks
	path, err := exp.Generate(a)
	if err != nil {
		t.Fatalf("Generate with no risks: %v", err)
	}
	CleanupFile(path)
}

func TestPDFExporter_WithAllRiskFlags(t *testing.T) {
	tmpDir := t.TempDir()
	exp := NewPDFExporter(tmpDir)

	a := makeAssessment(t, 35.0, "Foundational Risk Zone")
	a.Result.Risks = []string{"CRITICAL_GAPS", "DATA_HIGH_RISK", "SECURITY_HIGH_RISK", "MATURITY_CAPPED"}
	path, err := exp.Generate(a)
	if err != nil {
		t.Fatalf("Generate with all risks: %v", err)
	}
	CleanupFile(path)
}

func TestPDFExporter_WithNoRecommendations(t *testing.T) {
	tmpDir := t.TempDir()
	exp := NewPDFExporter(tmpDir)

	a := makeAssessment(t, 91.0, "AI-Native")
	a.Result.Recommendations = nil
	path, err := exp.Generate(a)
	if err != nil {
		t.Fatalf("Generate with no recommendations: %v", err)
	}
	CleanupFile(path)
}

// ─────────────────────────────────────────────
// CleanupFile tests
// ─────────────────────────────────────────────

func TestCleanupFile_DeletesFile(t *testing.T) {
	f, err := os.CreateTemp("", "test-cleanup-*.pdf")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	path := f.Name()
	f.Close()

	CleanupFile(path)

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("expected file to be deleted by CleanupFile")
	}
}

func TestCleanupFile_EmptyPathNoOp(t *testing.T) {
	// Should not panic
	CleanupFile("")
}

func TestCleanupFile_NonExistentFileNoOp(t *testing.T) {
	// Should not panic or error
	CleanupFile("/tmp/definitely-does-not-exist-12345.pdf")
}
