// app/assessment/page.tsx
'use client';
import Link from 'next/link';
import { useAssessment } from '@/lib/store';
import { DomainCard } from '@/components/DomainCard';
import { ProgressHeader } from '@/components/ProgressHeader';

export default function AssessmentDashboard() {
  const { state, questionBank, bankLoading, backendAvailable } = useAssessment();

  const totalQ = questionBank.questions.length;
  const totalAnswered = questionBank.questions.filter(
    (q) => state.answers[q.id]?.score != null
  ).length;

  return (
    <div className="max-w-6xl mx-auto px-6 py-8">
      <div className="flex items-start justify-between mb-6 flex-wrap gap-4">
        <div>
          <h1 className="text-3xl font-bold mb-1">Assessment Dashboard</h1>
          <p className="text-gray-500">Complete all 6 domains to compute your AI readiness score.</p>
        </div>
        <div className="flex items-center gap-3">
          {/* Backend status indicator */}
          <span className={`text-xs font-semibold px-2.5 py-1 rounded-full
            ${backendAvailable ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-500'}`}>
            {backendAvailable ? '● Backend connected' : '○ Offline mode'}
          </span>
          <Link href="/review"
            className="inline-flex items-center gap-2 bg-blue-600 hover:bg-blue-700 text-white font-semibold
              px-5 py-2.5 rounded-lg transition-all">
            → Review &amp; Compute
          </Link>
        </div>
      </div>

      {bankLoading && (
        <div className="bg-blue-50 border border-blue-200 text-blue-700 rounded-lg p-3 flex gap-2 items-center mb-4 text-sm">
          <svg className="animate-spin w-4 h-4" viewBox="0 0 24 24" fill="none">
            <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"/>
            <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z"/>
          </svg>
          Loading latest question bank from backend…
        </div>
      )}

      <ProgressHeader answered={totalAnswered} total={totalQ} />

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
        {questionBank.domains.map((domain) => (
          <DomainCard key={domain} domain={domain} answers={state.answers} questionBank={questionBank} />
        ))}
      </div>
    </div>
  );
}
