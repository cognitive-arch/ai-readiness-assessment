// app/review/page.tsx
'use client';
import { useState } from 'react';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { useAssessment } from '@/lib/store';
import { computeScores } from '@/lib/scoring';
import { api } from '@/lib/api';

const DOMAIN_META: Record<string, { color: string; icon: string }> = {
  Strategic:    { color: '#3b82f6', icon: '🎯' },
  Technology:   { color: '#8b5cf6', icon: '⚙️' },
  Data:         { color: '#06b6d4', icon: '🗄️' },
  Organization: { color: '#f59e0b', icon: '👥' },
  Security:     { color: '#ef4444', icon: '🔒' },
  UseCase:      { color: '#10b981', icon: '💡' },
};

export default function ReviewPage() {
  const { state, questionBank, setResult, backendAvailable, ensureAssessmentId } = useAssessment();
  const router = useRouter();
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [statusMsg, setStatusMsg] = useState('');

  const totalQ = questionBank.questions.length;
  const totalAnswered = questionBank.questions.filter(
    (q) => state.answers[q.id]?.score != null
  ).length;
  const confidence = Math.round((totalAnswered / totalQ) * 100);

  const handleCompute = async () => {
    setLoading(true);
    setError('');
    setStatusMsg('');

    try {
      if (backendAvailable) {
        // Step 1: ensure assessment exists on backend
        setStatusMsg('Creating assessment session…');
        const id = await ensureAssessmentId();

        // Step 2: bulk-save all current answers to backend
        setStatusMsg('Syncing answers to backend…');
        await api.saveAnswers(id, state.answers);

        // Step 3: run scoring engine on backend
        setStatusMsg('Computing results…');
        const result = await api.computeResults(id);
        setResult(result);
      } else {
        // Offline fallback: compute entirely client-side
        setStatusMsg('Computing results (offline mode)…');
        const result = computeScores(state.answers, questionBank);
        setResult(result);
      }

      router.push('/results');
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : 'Computation failed';
      console.error('Compute failed:', e);

      // Try client-side fallback if backend failed mid-flight
      try {
        setStatusMsg('Backend error — computing locally as fallback…');
        const result = computeScores(state.answers, questionBank);
        setResult(result);
        router.push('/results');
      } catch {
        setError(`${msg}. Client-side fallback also failed.`);
      }
    } finally {
      setLoading(false);
      setStatusMsg('');
    }
  };

  return (
    <div className="max-w-3xl mx-auto px-6 py-8">
      <h1 className="text-3xl font-bold mb-2">Review &amp; Compute</h1>
      <p className="text-gray-500 mb-6">Review your coverage before computing your AI readiness score.</p>

      {/* Backend status */}
      <div className={`flex items-center gap-2 px-4 py-2 rounded-lg text-sm font-medium mb-6
        ${backendAvailable ? 'bg-green-50 text-green-700 border border-green-200' : 'bg-gray-50 text-gray-500 border border-gray-200'}`}>
        <span className={`inline-block w-2 h-2 rounded-full ${backendAvailable ? 'bg-green-500' : 'bg-gray-400'}`} />
        {backendAvailable
          ? 'Backend connected — results will be computed server-side and saved'
          : 'Offline mode — results will be computed locally in the browser'}
      </div>

      {confidence < 80 && (
        <div className="bg-yellow-50 border border-yellow-200 text-yellow-800 rounded-lg p-4 flex gap-3 mb-6 text-sm">
          <span>⚠</span>
          <span>Coverage is {confidence}%. Results are less reliable below 80% completion.</span>
        </div>
      )}

      {/* Domain coverage grid */}
      <div className="bg-white border border-gray-200 rounded-xl overflow-hidden mb-8 shadow-sm">
        <div className="px-6 py-4 border-b border-gray-100">
          <h2 className="text-base font-semibold text-gray-800">Coverage Summary</h2>
        </div>
        <div className="grid grid-cols-1 sm:grid-cols-2">
          {questionBank.domains.map((d) => {
            const qs = questionBank.questions.filter(q => q.domain === d);
            const ans = qs.filter(q => state.answers[q.id]?.score != null).length;
            const pct = Math.round((ans / qs.length) * 100);
            const meta = DOMAIN_META[d];
            return (
              <div key={d} className="px-6 py-4 border-b border-r border-gray-100">
                <div className="flex justify-between items-center mb-2">
                  <span className="font-semibold text-sm" style={{ color: meta.color }}>
                    {meta.icon} {d}
                  </span>
                  <span className="text-sm font-bold text-gray-700">{ans}/{qs.length}</span>
                </div>
                <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden mb-2">
                  <div className="h-full rounded-full transition-all"
                    style={{ width: `${pct}%`, background: meta.color }} />
                </div>
                <Link href={`/assessment/${encodeURIComponent(d)}`}
                  className="text-xs text-blue-600 hover:underline">Edit →</Link>
              </div>
            );
          })}
        </div>
      </div>

      {/* Compute */}
      <div className="text-center py-8">
        <div className="mb-3">
          <span className={`inline-flex items-center px-4 py-1.5 rounded-full text-sm font-semibold
            ${confidence >= 80 ? 'bg-green-100 text-green-700'
              : confidence >= 50 ? 'bg-yellow-100 text-yellow-700'
              : 'bg-red-100 text-red-700'}`}>
            Confidence: {confidence}%
          </span>
        </div>
        <p className="text-gray-500 text-sm mb-2">{totalAnswered} of {totalQ} questions answered</p>
        {statusMsg && (
          <p className="text-blue-600 text-sm mb-4 flex items-center justify-center gap-2">
            <svg className="animate-spin w-4 h-4" viewBox="0 0 24 24" fill="none">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"/>
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8z"/>
            </svg>
            {statusMsg}
          </p>
        )}
        {error && <p className="text-red-600 text-sm mb-4">{error}</p>}
        <button
          onClick={handleCompute}
          disabled={loading}
          className="inline-flex items-center gap-3 bg-blue-600 hover:bg-blue-700 disabled:opacity-50
            disabled:cursor-not-allowed text-white font-semibold px-10 py-4 rounded-xl text-base
            transition-all shadow-md hover:shadow-lg">
          {loading ? '⏳ Computing...' : '🧮 Compute AI Readiness Score'}
        </button>
      </div>
    </div>
  );
}
