// internal/service/pdf.go
package service

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jung-kurt/gofpdf"
	"github.com/yourorg/ai-readiness-backend/internal/models"
)

// PDFExporter generates PDF reports.
type PDFExporter struct {
	tmpDir string
}

// NewPDFExporter creates a PDFExporter that writes temp files to tmpDir.
func NewPDFExporter(tmpDir string) *PDFExporter {
	return &PDFExporter{tmpDir: tmpDir}
}

// Generate creates a PDF report for the given assessment and returns the file path.
// The caller is responsible for deleting the file after serving it.
func (e *PDFExporter) Generate(a *models.Assessment) (string, error) {
	if a.Result == nil {
		return "", fmt.Errorf("assessment %s has no computed result", a.ID.Hex())
	}
	r := a.Result

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.SetMargins(20, 20, 20)
	pdf.AddPage()

	const (
		pageW  = 170.0 // usable width (210 - 20 - 20)
		col1   = 100.0
		col2   = 70.0
		lineH  = 7.0
		smallH = 5.5
	)

	// ── Header ──────────────────────────────────────────────
	pdf.SetFont("Helvetica", "B", 22)
	pdf.SetTextColor(37, 99, 235) // blue-600
	pdf.CellFormat(pageW, 12, "AI Readiness Assessment Report", "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(107, 114, 128) // gray-500
	pdf.CellFormat(pageW, 6, fmt.Sprintf("Assessment ID: %s", a.ID.Hex()), "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 6, fmt.Sprintf("Generated: %s", time.Now().UTC().Format("02 Jan 2006, 15:04 UTC")), "", 1, "R", false, 0, "")
	pdf.Ln(4)
	drawHLine(pdf, pageW)
	pdf.Ln(4)

	// ── Score summary ────────────────────────────────────────
	sectionHeader(pdf, pageW, "Overall Score")

	// Score circle (drawn as filled arc approximation via rectangle + text)
	scoreColor := scoreToRGB(r.Overall)
	pdf.SetFillColor(scoreColor[0], scoreColor[1], scoreColor[2])
	pdf.SetTextColor(255, 255, 255)
	pdf.SetFont("Helvetica", "B", 28)
	pdf.SetXY(20, pdf.GetY())
	pdf.CellFormat(38, 18, fmt.Sprintf("%.0f", r.Overall), "0", 0, "C", true, 0, "")

	pdf.SetTextColor(31, 41, 55) // gray-800
	pdf.SetFont("Helvetica", "B", 13)
	pdf.SetXY(62, pdf.GetY()-18)
	pdf.CellFormat(pageW-42, 9, r.Maturity, "", 1, "L", false, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	pdf.SetTextColor(107, 114, 128)
	pdf.SetX(62)
	pdf.CellFormat(pageW-42, 8, fmt.Sprintf("Confidence: %.0f%%   |   %d / %d questions answered",
		r.Confidence, r.TotalAnswered, r.TotalQ), "", 1, "L", false, 0, "")
	pdf.Ln(3)

	// ── Risk flags ───────────────────────────────────────────
	if len(r.Risks) > 0 {
		sectionHeader(pdf, pageW, "Risk Flags")
		riskLabels := map[string]string{
			"CRITICAL_GAPS":      "Critical Gaps — one or more critical questions scored ≤ 2",
			"DATA_HIGH_RISK":     "Data High Risk — Data domain score < 50",
			"SECURITY_HIGH_RISK": "Security High Risk — Security domain score < 50",
			"MATURITY_CAPPED":    "Maturity Capped — limited by Data or Security gaps",
		}
		pdf.SetFillColor(254, 226, 226) // red-100
		pdf.SetTextColor(185, 28, 28)   // red-700
		pdf.SetFont("Helvetica", "", 10)
		for _, risk := range r.Risks {
			label := riskLabels[risk]
			if label == "" {
				label = risk
			}
			pdf.SetX(20)
			pdf.CellFormat(pageW, smallH, "⚠  "+label, "0", 1, "L", false, 0, "")
		}
		pdf.Ln(3)
	}

	// ── Domain scores ────────────────────────────────────────
	sectionHeader(pdf, pageW, "Domain Scores")
	domainOrder := []string{"Strategic", "Technology", "Data", "Organization", "Security", "UseCase"}

	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetTextColor(107, 114, 128)
	pdf.SetFillColor(249, 250, 251)
	pdf.CellFormat(col1, smallH, "Domain", "1", 0, "L", true, 0, "")
	pdf.CellFormat(col2/3, smallH, "Score", "1", 0, "C", true, 0, "")
	pdf.CellFormat(col2/3, smallH, "Answered", "1", 0, "C", true, 0, "")
	pdf.CellFormat(col2/3, smallH, "Coverage", "1", 1, "C", true, 0, "")

	pdf.SetFont("Helvetica", "", 10)
	for _, d := range domainOrder {
		ds, ok := r.DomainScores[d]
		if !ok {
			continue
		}
		coverage := 0.0
		if ds.Total > 0 {
			coverage = float64(ds.Answered) / float64(ds.Total) * 100
		}
		rgb := scoreToRGB(ds.Score)
		pdf.SetTextColor(rgb[0], rgb[1], rgb[2])
		pdf.SetFillColor(255, 255, 255)
		pdf.CellFormat(col1, lineH, "  "+d, "1", 0, "L", true, 0, "")
		pdf.SetTextColor(31, 41, 55)
		pdf.CellFormat(col2/3, lineH, fmt.Sprintf("%.1f", ds.Score), "1", 0, "C", true, 0, "")
		pdf.CellFormat(col2/3, lineH, fmt.Sprintf("%d/%d", ds.Answered, ds.Total), "1", 0, "C", true, 0, "")
		pdf.CellFormat(col2/3, lineH, fmt.Sprintf("%.0f%%", coverage), "1", 1, "C", true, 0, "")
	}
	pdf.Ln(4)

	// ── Recommendations ──────────────────────────────────────
	pdf.AddPage()
	sectionHeader(pdf, pageW, fmt.Sprintf("Top %d Recommendations", len(r.Recommendations)))

	phases := []string{"Phase 1", "Phase 2", "Phase 3"}
	phaseTimeline := map[string]string{
		"Phase 1": "0–3 months",
		"Phase 2": "3–9 months",
		"Phase 3": "9–18 months",
	}
	phaseRGB := map[string][3]int{
		"Phase 1": {239, 68, 68},
		"Phase 2": {245, 158, 11},
		"Phase 3": {34, 197, 94},
	}

	for _, phase := range phases {
		var items []models.Recommendation
		for _, rec := range r.Recommendations {
			if rec.Phase == phase {
				items = append(items, rec)
			}
		}
		if len(items) == 0 {
			continue
		}

		rgb := phaseRGB[phase]
		pdf.SetFont("Helvetica", "B", 11)
		pdf.SetTextColor(rgb[0], rgb[1], rgb[2])
		pdf.CellFormat(pageW, 8, fmt.Sprintf("%s — %s", phase, phaseTimeline[phase]), "", 1, "L", false, 0, "")
		drawHLineColor(pdf, pageW, rgb[0], rgb[1], rgb[2])
		pdf.Ln(1)

		for i, rec := range items {
			pdf.SetFont("Helvetica", "", 10)
			pdf.SetTextColor(31, 41, 55)
			num := fmt.Sprintf("%02d. ", i+1)
			text := num + rec.Text

			// Priority badge inline
			priorRGB := priorityRGB(rec.Priority)
			pdf.SetFillColor(priorRGB[0], priorRGB[1], priorRGB[2])
			pdf.SetX(20)
			pdf.MultiCell(pageW-30, 5.5, text, "", "L", false)

			pdf.SetFont("Helvetica", "I", 9)
			pdf.SetTextColor(107, 114, 128)
			pdf.SetX(28)
			pdf.CellFormat(pageW-28, 5, fmt.Sprintf("[%s] [%s]", rec.Priority, rec.Domain), "", 1, "L", false, 0, "")
			pdf.Ln(1)
		}
		pdf.Ln(3)
	}

	// ── Footer ───────────────────────────────────────────────
	pdf.SetY(-20)
	pdf.SetFont("Helvetica", "I", 8)
	pdf.SetTextColor(156, 163, 175)
	pdf.CellFormat(pageW, 5, "AI Readiness Assessment Framework — Confidential", "", 0, "C", false, 0, "")

	// Write to temp file
	filename := fmt.Sprintf("assessment-%s-%d.pdf", a.ID.Hex(), time.Now().UnixMilli())
	path := filepath.Join(e.tmpDir, filename)
	if err := pdf.OutputFileAndClose(path); err != nil {
		return "", fmt.Errorf("write PDF: %w", err)
	}
	return path, nil
}

// ─────────────────────────────────────────────
// PDF drawing helpers
// ─────────────────────────────────────────────

func sectionHeader(pdf *gofpdf.Fpdf, pageW float64, title string) {
	pdf.SetFont("Helvetica", "B", 12)
	pdf.SetTextColor(17, 24, 39)
	pdf.SetFillColor(243, 244, 246)
	pdf.CellFormat(pageW, 8, "  "+title, "", 1, "L", true, 0, "")
	pdf.Ln(2)
}

func drawHLine(pdf *gofpdf.Fpdf, pageW float64) {
	drawHLineColor(pdf, pageW, 229, 231, 235)
}

func drawHLineColor(pdf *gofpdf.Fpdf, pageW float64, r, g, b int) {
	pdf.SetDrawColor(r, g, b)
	x := pdf.GetX()
	y := pdf.GetY()
	pdf.Line(x, y, x+pageW, y)
}

func scoreToRGB(score float64) [3]int {
	switch {
	case score >= 75:
		return [3]int{21, 128, 61}   // green-700
	case score >= 60:
		return [3]int{29, 78, 216}   // blue-700
	case score >= 40:
		return [3]int{161, 98, 7}    // yellow-700
	default:
		return [3]int{185, 28, 28}   // red-700
	}
}

func priorityRGB(priority string) [3]int {
	switch priority {
	case "Critical":
		return [3]int{254, 226, 226}
	case "High":
		return [3]int{255, 237, 213}
	default:
		return [3]int{254, 249, 195}
	}
}

// CleanupFile removes a temp PDF file, logging any error.
func CleanupFile(path string) {
	if path == "" {
		return
	}
	_ = os.Remove(path)
}

// barWidth maps a 0-100 score to a PDF bar width given max width.
func barWidth(score, maxW float64) float64 {
	return math.Min(score/100.0, 1.0) * maxW
}

// wrapText is a simple utility that truncates long strings for PDF cells.
func wrapText(s string, maxLen int) string {
	r := []rune(s)
	if len(r) > maxLen {
		return string(r[:maxLen-3]) + "..."
	}
	return s
}

// domainRows returns domains sorted by their canonical order.
func domainRows(scores map[string]models.DomainScore) []string {
	order := []string{"Strategic", "Technology", "Data", "Organization", "Security", "UseCase"}
	var out []string
	for _, d := range order {
		if _, ok := scores[d]; ok {
			out = append(out, d)
		}
	}
	// Append any unexpected domains
	for d := range scores {
		found := false
		for _, o := range order {
			if o == d {
				found = true
				break
			}
		}
		if !found {
			out = append(out, d)
		}
	}
	return out
}

// suppress unused-import errors for math/strings packages
var _ = strings.Contains
var _ = math.Abs
