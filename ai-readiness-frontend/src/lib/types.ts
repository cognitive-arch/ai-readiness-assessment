// lib/types.ts

export interface Question {
  id: string;
  domain: string;
  text: string;
  weight: number;
  isCritical: boolean;
  impact?: number;
}

export interface QuestionBank {
  domains: string[];
  questions: Question[];
}

export interface AnswerValue {
  score?: number;
  comment?: string;
}

export type Answers = Record<string, AnswerValue | undefined>;

export interface DomainScore {
  score: number;
  answered: number;
  total: number;
}

export type DomainScores = Record<string, DomainScore>;

export type MaturityLevel =
  | 'Foundational Risk Zone'
  | 'AI Emerging'
  | 'AI Structured'
  | 'AI Advanced'
  | 'AI-Native';

export interface Recommendation {
  domain: string;
  text: string;
  priority: 'Critical' | 'High' | 'Medium';
  phase: 'Phase 1' | 'Phase 2' | 'Phase 3';
}

export interface AssessmentResult {
  overall: number;
  domainScores: DomainScores;
  maturity: MaturityLevel;
  confidence: number;
  risks: string[];
  recommendations: Recommendation[];
  totalAnswered: number;
  totalQ: number;
}

export interface AssessmentState {
  assessmentId?: string;
  answers: Answers;
  result?: AssessmentResult;
  // Live question bank loaded from API (overrides bundled JSON when available)
  questionBank?: QuestionBank;
}
