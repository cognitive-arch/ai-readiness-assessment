// internal/scoring/engine.go
package scoring

import (
	"sort"

	"github.com/yourorg/ai-readiness-backend/internal/models"
)

// DomainWeights mirrors the frontend constants exactly.
var DomainWeights = map[string]float64{
	"Strategic":    0.20,
	"Technology":   0.20,
	"Data":         0.20,
	"Organization": 0.15,
	"Security":     0.15,
	"UseCase":      0.10,
}

// RecTemplates mirrors the frontend recommendation templates.
var RecTemplates = map[string][]string{
	"Strategic": {
		"Establish an AI governance committee with executive sponsorship",
		"Define measurable AI KPIs aligned to business strategy",
		"Develop a 3-year AI roadmap with phased delivery milestones",
	},
	"Technology": {
		"Implement MLOps pipeline for end-to-end model lifecycle management",
		"Deploy model monitoring and data drift detection tooling",
		"Establish cloud-native scalable infrastructure for AI workloads",
	},
	"Data": {
		"Implement a unified data catalog with lineage and quality tracking",
		"Deploy automated data quality monitoring and alerting pipelines",
		"Establish a data privacy and consent management framework",
	},
	"Organization": {
		"Create an AI Center of Excellence with a defined charter",
		"Launch organization-wide AI literacy upskilling program",
		"Implement cross-functional AI squad model for project delivery",
	},
	"Security": {
		"Conduct AI-specific threat modeling and security reviews",
		"Implement model explainability for all high-risk AI decisions",
		"Develop and test an AI incident response playbook",
	},
	"UseCase": {
		"Build a prioritized AI use case backlog with ROI estimates",
		"Deploy first high-value AI use case to production",
		"Establish quarterly use case portfolio review with business stakeholders",
	},
}

// Compute runs the full scoring algorithm against the provided answers
// and question bank, returning a populated Result.
func Compute(bank *models.QuestionBank, answers map[string]models.Answer) *models.Result {
	domainScores := computeDomainScores(bank, answers)
	overall := computeOverall(domainScores)
	maturity := classifyMaturity(overall)
	risks := detectRisks(bank, answers, domainScores, overall, &maturity)
	recommendations := generateRecommendations(domainScores, bank.Domains)

	totalAnswered := 0
	for _, a := range answers {
		if a.Score != nil {
			totalAnswered++
		}
	}
	confidence := 0.0
	if len(bank.Questions) > 0 {
		confidence = float64(totalAnswered) / float64(len(bank.Questions)) * 100
	}

	return &models.Result{
		Overall:         roundTwo(overall),
		DomainScores:    domainScores,
		Maturity:        maturity,
		Confidence:      roundTwo(confidence),
		Risks:           risks,
		Recommendations: recommendations,
		TotalAnswered:   totalAnswered,
		TotalQ:          len(bank.Questions),
	}
}

// Private helpers

func computeDomainScores(bank *models.QuestionBank, answers map[string]models.Answer) map[string]models.DomainScore {
	scores := make(map[string]models.DomainScore, len(bank.Domains))

	for _, domain := range bank.Domains {
		var totalWeight, weightedSum float64
		answered := 0
		total := 0

		for _, q := range bank.Questions {
			if q.Domain != domain {
				continue
			}
			total++
			ans, ok := answers[q.ID]
			if !ok || ans.Score == nil {
				continue
			}
			// Normalize 1-5 -> 0-100: ((score - 1) / 4) * 100
			normalized := (float64(*ans.Score-1) / 4.0) * 100.0
			weightedSum += normalized * float64(q.Weight)
			totalWeight += float64(q.Weight)
			answered++
		}

		domainScore := 0.0
		if totalWeight > 0 {
			domainScore = weightedSum / totalWeight
		}
		scores[domain] = models.DomainScore{
			Score:    roundTwo(domainScore),
			Answered: answered,
			Total:    total,
		}
	}
	return scores
}

func computeOverall(domainScores map[string]models.DomainScore) float64 {
	overall := 0.0
	for domain, ds := range domainScores {
		w, ok := DomainWeights[domain]
		if !ok {
			continue
		}
		overall += ds.Score * w
	}
	return overall
}

func classifyMaturity(score float64) string {
	switch {
	case score < 40:
		return "Foundational Risk Zone"
	case score < 60:
		return "AI Emerging"
	case score < 75:
		return "AI Structured"
	case score < 90:
		return "AI Advanced"
	default:
		return "AI-Native"
	}
}

func detectRisks(
	bank *models.QuestionBank,
	answers map[string]models.Answer,
	domainScores map[string]models.DomainScore,
	overall float64,
	maturity *string,
) []string {
	seen := map[string]bool{}
	var risks []string

	addRisk := func(r string) {
		if !seen[r] {
			seen[r] = true
			risks = append(risks, r)
		}
	}

	// CRITICAL_GAPS — any critical question scored <= 2
	for _, q := range bank.Questions {
		if !q.IsCritical {
			continue
		}
		ans, ok := answers[q.ID]
		if !ok || ans.Score == nil {
			continue
		}
		if *ans.Score <= 2 {
			addRisk("CRITICAL_GAPS")
			break
		}
	}

	// DATA_HIGH_RISK
	if ds, ok := domainScores["Data"]; ok && ds.Score < 50 {
		addRisk("DATA_HIGH_RISK")
	}

	// SECURITY_HIGH_RISK
	if ds, ok := domainScores["Security"]; ok && ds.Score < 50 {
		addRisk("SECURITY_HIGH_RISK")
	}

	// MATURITY_CAPPED — cap maturity if overall ≥ 75 but Data or Security < 50
	dataTooLow := domainScores["Data"].Score < 50
	secTooLow := domainScores["Security"].Score < 50
	if overall >= 75 && (dataTooLow || secTooLow) {
		*maturity = "AI Structured"
		addRisk("MATURITY_CAPPED")
	}

	return risks
}

func generateRecommendations(domainScores map[string]models.DomainScore, domains []string) []models.Recommendation {
	var recs []models.Recommendation

	for _, domain := range domains {
		ds, ok := domainScores[domain]
		if !ok {
			continue
		}

		priority := scoreToPriority(ds.Score)
		phase := scoreToPhase(ds.Score)
		count := 1
		if ds.Score < 50 {
			count = 3
		}

		templates := RecTemplates[domain]
		for i := 0; i < count && i < len(templates); i++ {
			recs = append(recs, models.Recommendation{
				Domain:   domain,
				Text:     templates[i],
				Priority: priority,
				Phase:    phase,
			})
		}
	}

	// Sort: Phase 1 first, then by priority (Critical → High → Medium)
	sort.Slice(recs, func(i, j int) bool {
		pi, pj := phaseOrder(recs[i].Phase), phaseOrder(recs[j].Phase)
		if pi != pj {
			return pi < pj
		}
		return priorityOrder(recs[i].Priority) < priorityOrder(recs[j].Priority)
	})

	// Cap at 15
	if len(recs) > 15 {
		recs = recs[:15]
	}
	return recs
}

func scoreToPriority(score float64) string {
	switch {
	case score < 40:
		return "Critical"
	case score < 65:
		return "High"
	default:
		return "Medium"
	}
}

func scoreToPhase(score float64) string {
	switch {
	case score < 40:
		return "Phase 1"
	case score < 65:
		return "Phase 2"
	default:
		return "Phase 3"
	}
}

func phaseOrder(p string) int {
	switch p {
	case "Phase 1":
		return 0
	case "Phase 2":
		return 1
	default:
		return 2
	}
}

func priorityOrder(p string) int {
	switch p {
	case "Critical":
		return 0
	case "High":
		return 1
	default:
		return 2
	}
}

func roundTwo(f float64) float64 {
	return float64(int(f*100+0.5)) / 100
}
