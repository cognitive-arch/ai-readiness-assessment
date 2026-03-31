'use client';
// components/DomainCard.tsx
import Link from 'next/link';
import type { Answers, QuestionBank } from '@/lib/types';

const DOMAIN_META: Record<string, { icon: string; color: string; bg: string; desc: string }> = {
  Strategic:    { icon: '🎯', color: '#3b82f6', bg: '#eff6ff', desc: 'AI vision, sponsorship, governance' },
  Technology:   { icon: '⚙️', color: '#8b5cf6', bg: '#f5f3ff', desc: 'Infrastructure, MLOps, engineering' },
  Data:         { icon: '🗄️', color: '#06b6d4', bg: '#ecfeff', desc: 'Data governance, quality, access' },
  Organization: { icon: '👥', color: '#f59e0b', bg: '#fffbeb', desc: 'Talent, culture, change management' },
  Security:     { icon: '🔒', color: '#ef4444', bg: '#fef2f2', desc: 'Security, privacy, compliance' },
  UseCase:      { icon: '💡', color: '#10b981', bg: '#ecfdf5', desc: 'Use case maturity & value delivery' },
};

interface Props {
  domain: string;
  answers: Answers;
  questionBank: QuestionBank;
}

export function DomainCard({ domain, answers, questionBank }: Props) {
  const meta = DOMAIN_META[domain] ?? { icon: '📋', color: '#6b7280', bg: '#f9fafb', desc: domain };
  const qs = questionBank.questions.filter((q) => q.domain === domain);
  const answered = qs.filter((q) => answers[q.id]?.score != null).length;
  const pct = qs.length > 0 ? Math.round((answered / qs.length) * 100) : 0;
  const status = answered === 0 ? 'empty' : answered === qs.length ? 'complete' : 'partial';

  const STATUS_STYLES = {
    complete: { dot: 'bg-green-500', label: 'Complete',     labelCls: 'text-green-600' },
    partial:  { dot: 'bg-yellow-400', label: 'In Progress', labelCls: 'text-yellow-600' },
    empty:    { dot: 'bg-gray-300',  label: 'Not Started',  labelCls: 'text-gray-400' },
  };
  const ss = STATUS_STYLES[status];

  return (
    <Link href={`/assessment/${encodeURIComponent(domain)}`}
      className="block bg-white border border-gray-200 rounded-xl shadow-sm hover:shadow-md
        hover:-translate-y-0.5 transition-all p-5 cursor-pointer">
      <div className="flex items-center justify-between mb-4">
        <div className="w-11 h-11 rounded-xl flex items-center justify-center text-xl"
          style={{ background: meta.bg }}>
          {meta.icon}
        </div>
        <div className="flex items-center gap-1.5">
          <div className={`w-2 h-2 rounded-full ${ss.dot}`} />
          <span className={`text-xs font-semibold ${ss.labelCls}`}>{ss.label}</span>
        </div>
      </div>
      <h3 className="font-bold text-base mb-1" style={{ color: meta.color }}>{domain}</h3>
      <p className="text-sm text-gray-400 mb-4 leading-snug">{meta.desc}</p>
      <div className="flex items-center justify-between text-xs text-gray-400 mb-2">
        <span>{answered} / {qs.length} answered</span>
        <span className="font-bold" style={{ color: meta.color }}>{pct}%</span>
      </div>
      <div className="h-1.5 bg-gray-100 rounded-full overflow-hidden mb-4">
        <div className="h-full rounded-full transition-all"
          style={{ width: `${pct}%`, background: meta.color }} />
      </div>
      <div className="border border-gray-200 bg-gray-50 hover:bg-gray-100 text-gray-700
        text-sm font-medium py-2 rounded-lg text-center transition-all">
        {answered === 0 ? 'Start →' : answered === qs.length ? 'Review →' : 'Continue →'}
      </div>
    </Link>
  );
}
