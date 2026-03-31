// lib/scoring.ts
// Client-side scoring engine — exact parity with the Go backend.
// Used as a fallback when the backend is unavailable.
// Now accepts the question bank as a parameter instead of importing it statically.

import type { Answers, DomainScores, AssessmentResult, QuestionBank, MaturityLevel } from './types';
import localBank from '../../public/question-bank-v1.json';

export const DOMAIN_WEIGHTS: Record<string, number> = {
  Strategic:    0.20,
  Technology:   0.20,
  Data:         0.20,
  Organization: 0.15,
  Security:     0.15,
  UseCase:      0.10,
};

export function classifyMaturity(score: number): MaturityLevel {
  if (score < 40) return 'Foundational Risk Zone';
  if (score < 60) return 'AI Emerging';
  if (score < 75) return 'AI Structured';
  if (score < 90) return 'AI Advanced';
  return 'AI-Native';
}

/**
 * Compute the full assessment result client-side.
 * @param answers   The current answers map from the store.
 * @param bank      Question bank (from API or local fallback). Defaults to bundled JSON.
 */
export function computeScores(
  answers: Answers,
  bank: QuestionBank = localBank as QuestionBank,
): AssessmentResult {
  const domainScores: DomainScores = {};

  for (const domain of bank.domains) {
    const qs = bank.questions.filter((q) => q.domain === domain);
    let totalWeight = 0;
    let weightedSum = 0;
    let answered = 0;

    for (const q of qs) {
      const a = answers[q.id];
      if (a?.score != null) {
        const normalized = ((a.score - 1) / 4) * 100;
        weightedSum += normalized * q.weight;
        totalWeight += q.weight;
        answered++;
      }
    }

    domainScores[domain] = {
      score: totalWeight > 0 ? round2(weightedSum / totalWeight) : 0,
      answered,
      total: qs.length,
    };
  }

  // Weighted overall score
  let overall = 0;
  for (const domain of bank.domains) {
    overall += domainScores[domain].score * (DOMAIN_WEIGHTS[domain] ?? 0);
  }
  overall = round2(overall);

  // Confidence
  const totalQ = bank.questions.length;
  const totalAnswered = Object.values(answers).filter((a) => a?.score != null).length;
  const confidence = round2((totalAnswered / totalQ) * 100);

  // Maturity
  let maturity = classifyMaturity(overall);

  // Risk flags
  const risks: string[] = [];
  for (const q of bank.questions) {
    if (q.isCritical && answers[q.id]?.score != null && answers[q.id]!.score! <= 2) {
      if (!risks.includes('CRITICAL_GAPS')) risks.push('CRITICAL_GAPS');
    }
  }
  if (domainScores['Data']?.score < 50)     risks.push('DATA_HIGH_RISK');
  if (domainScores['Security']?.score < 50) risks.push('SECURITY_HIGH_RISK');
  if (overall >= 75 && (domainScores['Data']?.score < 50 || domainScores['Security']?.score < 50)) {
    maturity = 'AI Structured';
    if (!risks.includes('MATURITY_CAPPED')) risks.push('MATURITY_CAPPED');
  }

  // Recommendations
  const recommendations = generateRecommendations(domainScores, bank.domains);

  return {
    overall,
    domainScores,
    maturity,
    confidence,
    risks,
    recommendations,
    totalAnswered,
    totalQ,
  };
}

// ─────────────────────────────────────────────
// Recommendation generation
// ─────────────────────────────────────────────

const REC_TEMPLATES: Record<string, string[]> = {
  Strategic: [
    'Establish an AI governance committee with executive sponsorship',
    'Define measurable AI KPIs aligned to business strategy',
    'Develop a 3-year AI roadmap with phased delivery milestones',
  ],
  Technology: [
    'Implement MLOps pipeline for end-to-end model lifecycle management',
    'Deploy model monitoring and data drift detection tooling',
    'Establish cloud-native scalable infrastructure for AI workloads',
  ],
  Data: [
    'Implement a unified data catalog with lineage and quality tracking',
    'Deploy automated data quality monitoring and alerting pipelines',
    'Establish a data privacy and consent management framework',
  ],
  Organization: [
    'Create an AI Center of Excellence with a defined charter',
    'Launch organization-wide AI literacy upskilling program',
    'Implement cross-functional AI squad model for project delivery',
  ],
  Security: [
    'Conduct AI-specific threat modeling and security reviews',
    'Implement model explainability for all high-risk AI decisions',
    'Develop and test an AI incident response playbook',
  ],
  UseCase: [
    'Build a prioritized AI use case backlog with ROI estimates',
    'Deploy first high-value AI use case to production',
    'Establish quarterly use case portfolio review with business stakeholders',
  ],
};

function generateRecommendations(
  domainScores: DomainScores,
  domains: string[],
): AssessmentResult['recommendations'] {
  const recs: AssessmentResult['recommendations'] = [];

  for (const domain of domains) {
    const score = domainScores[domain]?.score ?? 0;
    const priority: 'Critical' | 'High' | 'Medium' =
      score < 40 ? 'Critical' : score < 65 ? 'High' : 'Medium';
    const phase: 'Phase 1' | 'Phase 2' | 'Phase 3' =
      score < 40 ? 'Phase 1' : score < 65 ? 'Phase 2' : 'Phase 3';
    const count = score < 50 ? 3 : 1;

    for (const text of (REC_TEMPLATES[domain] ?? []).slice(0, count)) {
      recs.push({ domain, text, priority, phase });
    }
  }

  return recs
    .sort((a, b) => {
      const phaseOrder = { 'Phase 1': 0, 'Phase 2': 1, 'Phase 3': 2 };
      const priOrder   = { 'Critical': 0, 'High': 1, 'Medium': 2 };
      return (phaseOrder[a.phase] - phaseOrder[b.phase])
          || (priOrder[a.priority] - priOrder[b.priority]);
    })
    .slice(0, 15);
}

function round2(n: number): number {
  return Math.round(n * 100) / 100;
}
