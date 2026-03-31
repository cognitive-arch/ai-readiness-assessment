'use client';
// components/ProgressHeader.tsx
interface Props {
  answered: number;
  total: number;
}
export function ProgressHeader({ answered, total }: Props) {
  const pct = total > 0 ? Math.round((answered / total) * 100) : 0;
  return (
    <div className="bg-white border border-gray-200 rounded-xl p-5 mb-5 flex items-center gap-5 shadow-sm">
      <span className="text-sm font-semibold text-gray-600 whitespace-nowrap">Overall Progress</span>
      <div className="flex-1 h-2 bg-gray-100 rounded-full overflow-hidden">
        <div className="h-full bg-blue-600 rounded-full transition-all duration-500" style={{ width: `${pct}%` }} />
      </div>
      <span className="text-xl font-extrabold text-blue-600 whitespace-nowrap" style={{ fontFamily: 'var(--font-mono)' }}>
        {pct}%
      </span>
      <span className="text-sm text-gray-400 whitespace-nowrap">{answered}/{total}</span>
    </div>
  );
}
