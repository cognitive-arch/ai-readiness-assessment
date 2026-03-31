// lib/api.ts
import type { Answers, AssessmentResult, Question } from './types';

// ─────────────────────────────────────────────
// Read BASE_URL lazily on every call — NOT at module load time.
// This prevents the value being frozen as `undefined` if Next.js
// hasn't yet resolved the env var when this module is first imported.
// ─────────────────────────────────────────────
function getBaseURL(): string {
  const url = process.env.NEXT_PUBLIC_API_BASE_URL ?? '';
  return url.replace(/\/$/, ''); // strip trailing slash
}

// ─────────────────────────────────────────────
// Backend response envelope
// All responses: { success: boolean, data: T, error?: string }
// ─────────────────────────────────────────────

interface ApiEnvelope<T> {
  success: boolean;
  data: T;
  error?: string;
}

async function apiFetch<T>(path: string, options?: RequestInit): Promise<T> {
  const url = `${getBaseURL()}${path}`;

  const res = await fetch(url, {
    headers: { 'Content-Type': 'application/json' },
    ...options,
  });

  let json: ApiEnvelope<T>;
  try {
    json = await res.json();
  } catch {
    throw new Error(`API error ${res.status}: non-JSON response from ${path}`);
  }

  if (!res.ok || !json.success) {
    throw new Error(json.error || `API error ${res.status} on ${path}`);
  }

  return json.data;
}

// ─────────────────────────────────────────────
// Backend payload shapes
// ─────────────────────────────────────────────

interface BackendAssessment {
  assessmentId: string;
  status: string;
  answers: Record<string, { score?: number; comment?: string }>;
  result?: BackendResult;
}

interface BackendResult {
  overall: number;
  domainScores: Record<string, { score: number; answered: number; total: number }>;
  maturity: string;
  confidence: number;
  risks: string[];
  recommendations: Array<{
    domain: string;
    text: string;
    priority: 'Critical' | 'High' | 'Medium';
    phase: 'Phase 1' | 'Phase 2' | 'Phase 3';
  }>;
  totalAnswered: number;
  totalQ: number;
}

export interface BackendQuestionBank {
  domains: string[];
  questions: Question[];
}

// ─────────────────────────────────────────────
// Helpers
// ─────────────────────────────────────────────

function toAssessmentResult(r: BackendResult): AssessmentResult {
  return {
    overall: r.overall,
    domainScores: r.domainScores,
    maturity: r.maturity as AssessmentResult['maturity'],
    confidence: r.confidence,
    risks: r.risks,
    recommendations: r.recommendations,
    totalAnswered: r.totalAnswered,
    totalQ: r.totalQ,
  };
}

// Strip unanswered entries and cast scores to integers before sending
function sanitiseAnswers(answers: Answers): Record<string, { score: number; comment?: string }> {
  const out: Record<string, { score: number; comment?: string }> = {};
  for (const [id, val] of Object.entries(answers)) {
    if (!val || val.score == null) continue;
    const entry: { score: number; comment?: string } = { score: Math.round(val.score) };
    if (val.comment?.trim()) entry.comment = val.comment.trim();
    out[id] = entry;
  }
  return out;
}

// ─────────────────────────────────────────────
// API client
// ─────────────────────────────────────────────

export const api = {
  /** Fetch question bank from the backend. Returns null if unavailable. */
  async getQuestions(): Promise<BackendQuestionBank | null> {
    if (!getBaseURL()) return null;
    try {
      return await apiFetch<BackendQuestionBank>('/api/questions');
    } catch {
      return null;
    }
  },

  /** Create a new assessment session. Returns the assessmentId string. */
  async createAssessment(clientRef?: string): Promise<string> {
    const data = await apiFetch<BackendAssessment>('/api/assessment', {
      method: 'POST',
      body: JSON.stringify({ client_ref: clientRef ?? '' }),
    });
    return data.assessmentId;
  },

  /** Bulk-save answers. No-ops if there are no scored answers. */
  async saveAnswers(assessmentId: string, answers: Answers): Promise<void> {
    const sanitised = sanitiseAnswers(answers);
    if (Object.keys(sanitised).length === 0) return;
    await apiFetch<BackendAssessment>(`/api/assessment/${assessmentId}/answers`, {
      method: 'PUT',
      body: JSON.stringify({ answers: sanitised }),
    });
  },

  /** Run the scoring engine on the backend. Returns AssessmentResult. */
  async computeResults(assessmentId: string): Promise<AssessmentResult> {
    const data = await apiFetch<BackendAssessment>(
      `/api/assessment/${assessmentId}/compute`,
      { method: 'POST', body: '{}' },
    );
    if (!data.result) throw new Error('Backend returned no result after compute');
    return toAssessmentResult(data.result);
  },

  /** Fetch previously computed results. */
  async getResults(assessmentId: string): Promise<AssessmentResult> {
    const data = await apiFetch<BackendResult>(`/api/assessment/${assessmentId}/results`);
    return toAssessmentResult(data);
  },

  /** Open PDF export in a new browser tab. */
  exportPDF(assessmentId: string): void {
    window.open(`${getBaseURL()}/api/assessment/${assessmentId}/export/pdf`, '_blank');
  },

  /** Check whether the backend API is reachable. */
  async isAvailable(): Promise<boolean> {
    const base = getBaseURL();
    if (!base) return false;
    try {
      const res = await fetch(`${base}/health`, {
        signal: AbortSignal.timeout(3000),
      });
      return res.ok;
    } catch {
      return false;
    }
  },
};
