// app/assessment/[domain]/page.tsx
'use client';
import { useParams } from 'next/navigation';
import Link from 'next/link';
import { useAssessment } from '@/lib/store';
import { QuestionItem } from '@/components/QuestionItem';
import { api } from '@/lib/api';
import { useCallback } from 'react';
import type { AnswerValue } from '@/lib/types';

const DOMAIN_META: Record<string, { icon: string; color: string; bg: string; desc: string }> = {
  Strategic:    { icon: '🎯', color: '#3b82f6', bg: '#eff6ff', desc: 'AI vision, sponsorship, governance, and strategic alignment' },
  Technology:   { icon: '⚙️', color: '#8b5cf6', bg: '#f5f3ff', desc: 'Infrastructure, MLOps pipelines, and engineering maturity' },
  Data:         { icon: '🗄️', color: '#06b6d4', bg: '#ecfeff', desc: 'Data governance, quality, access, and readiness for AI' },
  Organization: { icon: '👥', color: '#f59e0b', bg: '#fffbeb', desc: 'Talent, culture, change management, and AI literacy' },
  Security:     { icon: '🔒', color: '#ef4444', bg: '#fef2f2', desc: 'AI security, privacy, ethics, and regulatory compliance' },
  UseCase:      { icon: '💡', color: '#10b981', bg: '#ecfdf5', desc: 'Use case maturity, prioritization, and value delivery' },
};

export default function DomainPage() {
  const params = useParams();
  const { state, questionBank, setAnswer, ensureAssessmentId, backendAvailable } = useAssessment();

  const domain = decodeURIComponent(
    Array.isArray(params.domain) ? params.domain[0] : params.domain
  );

  const meta = DOMAIN_META[domain];
  const qs = questionBank.questions.filter((q) => q.domain === domain);
  const domainIndex = questionBank.domains.indexOf(domain);
  const answered = qs.filter((q) => state.answers[q.id]?.score != null).length;

  if (!meta) {
    return <div className="p-8 text-red-500">Domain not found: {domain}</div>;
  }

  const prevDomain = domainIndex > 0 ? questionBank.domains[domainIndex - 1] : null;
  const nextDomain = domainIndex < questionBank.domains.length - 1 ? questionBank.domains[domainIndex + 1] : null;

  // Handle answer change: update local state immediately, then sync to backend
  const handleAnswerChange = useCallback(async (questionId: string, answer: AnswerValue) => {
    setAnswer(questionId, answer);

    // Only sync answered questions (score must be present)
    if (!backendAvailable || answer.score == null) return;
    try {
      const id = await ensureAssessmentId();
      await api.saveAnswers(id, { [questionId]: answer });
    } catch {
      // Silently fail — answer is safely in localStorage, will be batch-saved on compute
    }
  }, [setAnswer, ensureAssessmentId, backendAvailable]);

  return (
    <div className="max-w-3xl mx-auto px-6 py-8">
      {/* Header */}
      <div className="flex items-center gap-3 mb-2">
        <Link href="/assessment" className="text-sm text-blue-600 hover:underline">← Dashboard</Link>
        <span className="text-xs bg-gray-100 text-gray-500 px-2.5 py-1 rounded-full font-semibold">
          Domain {domainIndex + 1} of {questionBank.domains.length}
        </span>
      </div>

      <div className="flex items-center gap-4 mb-6">
        <div className="w-14 h-14 rounded-xl flex items-center justify-center text-2xl" style={{ background: meta.bg }}>
          {meta.icon}
        </div>
        <div>
          <h1 className="text-2xl font-bold" style={{ color: meta.color }}>{domain}</h1>
          <p className="text-gray-500 text-sm">{meta.desc}</p>
        </div>
      </div>

      {/* Step dots */}
      <div className="flex items-center mb-6">
        {questionBank.domains.map((d, i) => {
          const isDone = i < domainIndex;
          const isActive = d === domain;
          return (
            <div key={d} className="flex items-center flex-1">
              <Link
                href={`/assessment/${encodeURIComponent(d)}`}
                title={d}
                className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold flex-shrink-0
                  transition-all ${isDone ? 'bg-green-500 text-white' : isActive ? 'bg-blue-600 text-white' : 'bg-gray-200 text-gray-500'}`}>
                {isDone ? '✓' : i + 1}
              </Link>
              {i < questionBank.domains.length - 1 && (
                <div className={`flex-1 h-0.5 ${isDone ? 'bg-green-500' : 'bg-gray-200'}`} />
              )}
            </div>
          );
        })}
      </div>

      {/* Coverage */}
      <div className="flex items-center justify-between mb-4">
        <span className="text-sm text-gray-500 font-semibold">{answered} of {qs.length} answered</span>
        {answered < qs.length && (
          <span className="text-xs bg-yellow-100 text-yellow-700 border border-yellow-200 px-2.5 py-1 rounded-full font-semibold">
            ⚠ {qs.length - answered} unanswered
          </span>
        )}
      </div>

      {/* Questions */}
      <div className="space-y-3">
        {qs.map((q) => (
          <QuestionItem
            key={q.id}
            question={q}
            answer={state.answers[q.id]}
            onChange={(ans) => handleAnswerChange(q.id, ans)}
          />
        ))}
      </div>

      {/* Navigation */}
      <div className="flex justify-between mt-8 pt-5 border-t border-gray-200">
        {prevDomain ? (
          <Link href={`/assessment/${encodeURIComponent(prevDomain)}`}
            className="inline-flex items-center gap-2 border border-gray-300 bg-white hover:bg-gray-50 text-gray-700 font-medium px-4 py-2.5 rounded-lg transition-all">
            ← {prevDomain}
          </Link>
        ) : (
          <Link href="/assessment"
            className="inline-flex items-center gap-2 border border-gray-300 bg-white hover:bg-gray-50 text-gray-700 font-medium px-4 py-2.5 rounded-lg transition-all">
            ← Dashboard
          </Link>
        )}
        <div className="flex gap-2">
          <Link href="/assessment"
            className="inline-flex items-center gap-2 text-gray-500 hover:text-gray-700 hover:bg-gray-100 font-medium px-4 py-2.5 rounded-lg transition-all">
            Save &amp; Exit
          </Link>
          {nextDomain ? (
            <Link href={`/assessment/${encodeURIComponent(nextDomain)}`}
              className="inline-flex items-center gap-2 bg-blue-600 hover:bg-blue-700 text-white font-semibold px-5 py-2.5 rounded-lg transition-all">
              Next: {nextDomain} →
            </Link>
          ) : (
            <Link href="/review"
              className="inline-flex items-center gap-2 bg-blue-600 hover:bg-blue-700 text-white font-semibold px-5 py-2.5 rounded-lg transition-all">
              Review &amp; Compute →
            </Link>
          )}
        </div>
      </div>
    </div>
  );
}
