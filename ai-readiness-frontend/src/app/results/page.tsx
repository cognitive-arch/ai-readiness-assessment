// app/results/page.tsx
'use client';
import { useState } from 'react';
import Link from 'next/link';
import { useAssessment } from '@/lib/store';
import { api } from '@/lib/api';
import { RadarChart } from '@/components/RadarChart';
import { RecommendationsList } from '@/components/RecommendationsList';
import { RoadmapBoard } from '@/components/RoadmapBoard';
import type { Answers, QuestionBank } from '@/lib/types';

// ─────────────────────────────────────────────
// Constants
// ─────────────────────────────────────────────

const MATURITY_BADGE: Record<string, string> = {
  'Foundational Risk Zone': 'bg-red-100 text-red-700',
  'AI Emerging':            'bg-orange-100 text-orange-700',
  'AI Structured':          'bg-yellow-100 text-yellow-700',
  'AI Advanced':            'bg-blue-100 text-blue-700',
  'AI-Native':              'bg-green-100 text-green-700',
};

const RISK_LABELS: Record<string, { label: string; cls: string }> = {
  CRITICAL_GAPS:      { label: 'Critical Gaps — critical questions scored ≤ 2', cls: 'bg-red-100 text-red-700' },
  DATA_HIGH_RISK:     { label: 'Data High Risk — Data domain score < 50',        cls: 'bg-orange-100 text-orange-700' },
  SECURITY_HIGH_RISK: { label: 'Security High Risk — Security domain score < 50', cls: 'bg-red-100 text-red-700' },
  MATURITY_CAPPED:    { label: 'Maturity Capped — Limited by Data or Security gap', cls: 'bg-yellow-100 text-yellow-700' },
};

const DOMAIN_META: Record<string, { color: string; icon: string }> = {
  Strategic:    { color: '#3b82f6', icon: '🎯' },
  Technology:   { color: '#8b5cf6', icon: '⚙️' },
  Data:         { color: '#06b6d4', icon: '🗄️' },
  Organization: { color: '#f59e0b', icon: '👥' },
  Security:     { color: '#ef4444', icon: '🔒' },
  UseCase:      { color: '#10b981', icon: '💡' },
};

const HEAT_COLORS: Record<number, string> = {
  1: 'bg-red-100 text-red-700',
  2: 'bg-orange-100 text-orange-700',
  3: 'bg-yellow-100 text-yellow-700',
  4: 'bg-green-100 text-green-700',
  5: 'bg-emerald-100 text-emerald-700',
};

const TABS = [
  { id: 'overview',         label: 'Overview' },
  { id: 'heatmap',          label: 'Gap Analysis' },
  { id: 'recommendations',  label: 'Recommendations' },
  { id: 'roadmap',          label: 'Roadmap' },
] as const;

type TabId = typeof TABS[number]['id'];

// ─────────────────────────────────────────────
// Score ring SVG
// ─────────────────────────────────────────────

function ScoreRing({ score }: { score: number }) {
  const size = 140;
  const r = (size - 20) / 2;
  const circ = 2 * Math.PI * r;
  const fill = Math.min(score / 100, 1) * circ;
  const color = score >= 75 ? '#22c55e' : score >= 60 ? '#3b82f6' : score >= 40 ? '#f59e0b' : '#ef4444';
  return (
    <div className="relative inline-flex items-center justify-center" style={{ width: size, height: size }}>
      <svg width={size} height={size} style={{ transform: 'rotate(-90deg)' }}>
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke="#f3f4f6" strokeWidth={10} />
        <circle cx={size / 2} cy={size / 2} r={r} fill="none" stroke={color} strokeWidth={10}
          strokeDasharray={`${fill} ${circ}`} strokeLinecap="round"
          style={{ transition: 'stroke-dasharray 1s ease' }} />
      </svg>
      <div className="absolute text-center">
        <div className="text-4xl font-black leading-none" style={{ color, fontFamily: 'var(--font-mono)' }}>
          {Math.round(score)}
        </div>
        <div className="text-xs text-gray-400 font-medium">/100</div>
      </div>
    </div>
  );
}

// ─────────────────────────────────────────────
// Heatmap — now receives questionBank from store (not static import)
// ─────────────────────────────────────────────

function HeatmapTable({
  answers,
  questionBank,
}: {
  answers: Answers;
  questionBank: QuestionBank;
}) {
  const hasAnyGaps = questionBank.domains.some(domain => {
    const qs = questionBank.questions.filter(q => q.domain === domain);
    return qs.some(q => {
      const score = answers[q.id]?.score;
      return score != null && score <= 3;
    });
  });

  if (!hasAnyGaps) {
    return (
      <div className="text-center py-12 text-gray-400">
        <div className="text-3xl mb-3">✅</div>
        <p className="font-medium text-gray-600">No gaps detected</p>
        <p className="text-sm mt-1">All answered questions scored 4 or higher.</p>
      </div>
    );
  }

  return (
    <div className="space-y-8">
      {questionBank.domains.map(domain => {
        const qs = questionBank.questions.filter(q => q.domain === domain);
        const gaps = qs.filter(q => {
          const score = answers[q.id]?.score;
          return score != null && score <= 2;
        });
        const meds = qs.filter(q => answers[q.id]?.score === 3);
        if (!gaps.length && !meds.length) return null;

        const meta = DOMAIN_META[domain] ?? { color: '#6b7280', icon: '📋' };
        return (
          <div key={domain}>
            <div
              className="text-sm font-bold uppercase tracking-wider mb-3 flex items-center gap-2 pb-2 border-b-2"
              style={{ color: meta.color, borderColor: meta.color }}>
              {meta.icon} {domain}
              {gaps.length > 0 && (
                <span className="text-red-500 normal-case font-semibold tracking-normal text-xs ml-1">
                  {gaps.length} critical gap{gaps.length !== 1 ? 's' : ''}
                </span>
              )}
              {meds.length > 0 && (
                <span className="text-yellow-600 normal-case font-semibold tracking-normal text-xs">
                  · {meds.length} improvement area{meds.length !== 1 ? 's' : ''}
                </span>
              )}
            </div>
            <div className="space-y-0">
              {[...gaps, ...meds].map(q => {
                const score = answers[q.id]?.score;
                return (
                  <div key={q.id} className="flex gap-3 items-start py-2.5 border-b border-gray-50 last:border-0">
                    <div className={`w-7 h-7 rounded flex-shrink-0 flex items-center justify-center text-xs font-bold
                      ${score != null ? HEAT_COLORS[score] : 'bg-gray-100 text-gray-400'}`}>
                      {score ?? '?'}
                    </div>
                    <div className="flex-1 min-w-0">
                      {q.isCritical && (
                        <span className="text-xs bg-red-100 text-red-700 px-2 py-0.5 rounded-full font-semibold mr-2">
                          ⚡ Critical
                        </span>
                      )}
                      <span className="text-sm text-gray-700">{q.text}</span>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        );
      })}
    </div>
  );
}

// ─────────────────────────────────────────────
// Main results page
// ─────────────────────────────────────────────

export default function ResultsPage() {
  const { state, questionBank, backendAvailable } = useAssessment();
  const [tab, setTab] = useState<TabId>('overview');
  const [pdfLoading, setPdfLoading] = useState(false);

  const result = state.result;

  if (!result) {
    return (
      <div className="max-w-xl mx-auto px-6 py-20 text-center">
        <div className="text-5xl mb-6">📊</div>
        <h1 className="text-2xl font-bold mb-4 text-gray-700">No results yet</h1>
        <p className="text-gray-500 mb-8">Complete the assessment and compute your score first.</p>
        <Link href="/assessment"
          className="inline-flex items-center gap-2 bg-blue-600 text-white font-semibold px-6 py-3 rounded-lg hover:bg-blue-700 transition-all">
          Start Assessment →
        </Link>
      </div>
    );
  }

  const { overall, domainScores, maturity, confidence, risks, recommendations } = result;
  const assessmentId = state.assessmentId;

  // ── Export handlers ──────────────────────────────────────

  const exportJSON = () => {
    const blob = new Blob([JSON.stringify(result, null, 2)], { type: 'application/json' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `ai-readiness-${assessmentId ?? 'results'}.json`;
    a.click();
    URL.revokeObjectURL(a.href);
  };

  const exportMarkdown = () => {
    const lines = [
      '# AI Readiness Assessment Results\n',
      `**Overall Score:** ${Math.round(overall)}/100`,
      `**Maturity Level:** ${maturity}`,
      `**Confidence:** ${Math.round(confidence)}%\n`,
      '## Domain Scores',
      ...Object.entries(domainScores).map(
        ([d, s]) => `- **${d}:** ${Math.round(s.score)}/100 (${s.answered}/${s.total} answered)`
      ),
      '\n## Risk Flags',
      risks.length
        ? risks.map(r => `- ⚠ ${RISK_LABELS[r]?.label || r}`).join('\n')
        : '- None',
      '\n## Top Recommendations',
      ...recommendations.map(
        (r, i) => `${i + 1}. [${r.phase}] [${r.priority}] ${r.text} *(${r.domain})*`
      ),
    ];
    const blob = new Blob([lines.join('\n')], { type: 'text/markdown' });
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = `ai-readiness-${assessmentId ?? 'report'}.md`;
    a.click();
    URL.revokeObjectURL(a.href);
  };

  const exportPDF = async () => {
    if (!assessmentId || !backendAvailable) {
      // Fallback: browser print dialog
      window.print();
      return;
    }
    setPdfLoading(true);
    try {
      api.exportPDF(assessmentId); // opens in new tab
    } finally {
      setPdfLoading(false);
    }
  };

  return (
    <div className="max-w-5xl mx-auto px-6 py-8">
      {/* Page header */}
      <div className="flex items-start justify-between mb-6 flex-wrap gap-4">
        <div>
          <h1 className="text-3xl font-bold mb-1">AI Readiness Results</h1>
          <p className="text-gray-500">
            {assessmentId
              ? <span>Assessment <code className="text-xs bg-gray-100 px-1.5 py-0.5 rounded">{assessmentId}</code></span>
              : 'Computed locally'}
          </p>
        </div>
        <div className="flex gap-2 flex-wrap">
          <button onClick={exportJSON}
            className="border border-gray-300 bg-white hover:bg-gray-50 text-gray-700 font-medium px-4 py-2 rounded-lg text-sm transition-all">
            ⬇ JSON
          </button>
          <button onClick={exportMarkdown}
            className="border border-gray-300 bg-white hover:bg-gray-50 text-gray-700 font-medium px-4 py-2 rounded-lg text-sm transition-all">
            ⬇ Markdown
          </button>
          <button
            onClick={exportPDF}
            disabled={pdfLoading}
            title={!backendAvailable ? 'Backend offline — will use browser print' : 'Download PDF report'}
            className="border border-gray-300 bg-white hover:bg-gray-50 text-gray-700 font-medium px-4 py-2 rounded-lg text-sm transition-all disabled:opacity-50">
            {pdfLoading ? '⏳ …' : `⬇ PDF${!backendAvailable ? ' (print)' : ''}`}
          </button>
          <Link href="/assessment"
            className="border border-gray-300 bg-white hover:bg-gray-50 text-gray-700 font-medium px-4 py-2 rounded-lg text-sm transition-all">
            New Assessment
          </Link>
        </div>
      </div>

      {/* Score hero card */}
      <div className="bg-white border border-gray-200 rounded-xl shadow-sm p-6 mb-5 flex items-center gap-8 flex-wrap">
        <ScoreRing score={overall} />

        <div className="flex-1 min-w-48">
          <div className="mb-4">
            <span className={`inline-flex items-center px-4 py-2 rounded-full text-base font-bold ${MATURITY_BADGE[maturity] ?? 'bg-gray-100 text-gray-700'}`}>
              {maturity}
            </span>
          </div>
          <div className="grid grid-cols-2 gap-4">
            <div>
              <div className="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-1">Confidence</div>
              <div className="text-2xl font-black text-gray-800" style={{ fontFamily: 'var(--font-mono)' }}>
                {Math.round(confidence)}%
              </div>
            </div>
            <div>
              <div className="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-1">Risk Flags</div>
              <div className={`text-2xl font-black ${risks.length > 0 ? 'text-red-500' : 'text-green-500'}`}
                style={{ fontFamily: 'var(--font-mono)' }}>
                {risks.length}
              </div>
            </div>
          </div>
        </div>

        {risks.length > 0 && (
          <div className="flex-1 min-w-48">
            <div className="text-xs font-semibold text-gray-400 uppercase tracking-wide mb-3">Active Risk Flags</div>
            <div className="flex flex-col gap-2">
              {risks.map(r => (
                <div key={r}
                  className={`inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-semibold ${RISK_LABELS[r]?.cls ?? 'bg-gray-100 text-gray-600'}`}>
                  ⚠ {RISK_LABELS[r]?.label ?? r}
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      {/* Tab bar */}
      <div className="flex border-b border-gray-200 mb-6 overflow-x-auto">
        {TABS.map(t => (
          <button key={t.id} onClick={() => setTab(t.id)}
            className={`px-5 py-3 text-sm font-medium border-b-2 -mb-px transition-all whitespace-nowrap
              ${tab === t.id
                ? 'border-blue-600 text-blue-600 font-semibold'
                : 'border-transparent text-gray-500 hover:text-gray-700'}`}>
            {t.label}
          </button>
        ))}
      </div>

      {/* ── Overview ─────────────────────────────────────── */}
      {tab === 'overview' && (
        <div className="grid grid-cols-1 md:grid-cols-2 gap-5">
          <RadarChart domainScores={domainScores} />

          <div className="bg-white border border-gray-200 rounded-xl shadow-sm">
            <div className="px-6 py-4 border-b border-gray-100">
              <h3 className="font-semibold text-gray-800">Domain Scores</h3>
            </div>
            <div className="p-6 space-y-4">
              {Object.entries(domainScores).map(([d, ds]) => {
                const meta = DOMAIN_META[d] ?? { color: '#6b7280', icon: '📋' };
                return (
                  <div key={d}>
                    <div className="flex items-center justify-between mb-1.5">
                      <span className="text-sm font-semibold" style={{ color: meta.color }}>
                        {meta.icon} {d}
                      </span>
                      <div className="flex items-center gap-2">
                        <span className="text-xs text-gray-400">{ds.answered}/{ds.total}</span>
                        <span className="text-base font-black"
                          style={{ color: meta.color, fontFamily: 'var(--font-mono)' }}>
                          {Math.round(ds.score)}
                        </span>
                      </div>
                    </div>
                    <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden">
                      <div className="h-full rounded-full transition-all"
                        style={{ width: `${ds.score}%`, background: meta.color }} />
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        </div>
      )}

      {/* ── Gap Analysis / Heatmap ────────────────────────── */}
      {tab === 'heatmap' && (
        <div className="bg-white border border-gray-200 rounded-xl shadow-sm">
          <div className="px-6 py-4 border-b border-gray-100">
            <h3 className="font-semibold text-gray-800">Question-Level Gap Analysis</h3>
            <p className="text-sm text-gray-500 mt-0.5">
              Scores 1–2 indicate critical gaps; score 3 indicates improvement opportunities.
            </p>
          </div>
          <div className="p-6">
            {/* Pass questionBank from store — no static import */}
            <HeatmapTable answers={state.answers} questionBank={questionBank} />
          </div>
        </div>
      )}

      {/* ── Recommendations ───────────────────────────────── */}
      {tab === 'recommendations' && <RecommendationsList recommendations={recommendations} />}

      {/* ── Roadmap ───────────────────────────────────────── */}
      {tab === 'roadmap' && <RoadmapBoard recommendations={recommendations} />}
    </div>
  );
}
