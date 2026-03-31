'use client';
// components/RecommendationsList.tsx
import type { Recommendation } from '@/lib/types';

const PRIORITY_BADGE: Record<string, string> = {
  Critical: 'bg-red-100 text-red-700',
  High:     'bg-orange-100 text-orange-700',
  Medium:   'bg-yellow-100 text-yellow-700',
};
const PHASE_BADGE = 'bg-blue-100 text-blue-700';
const DOMAIN_BADGE = 'bg-gray-100 text-gray-600';

interface Props {
  recommendations: Recommendation[];
}

export function RecommendationsList({ recommendations }: Props) {
  return (
    <div className="bg-white border border-gray-200 rounded-xl shadow-sm">
      <div className="px-6 py-4 border-b border-gray-100">
        <h3 className="font-semibold text-gray-800">Top {recommendations.length} Recommendations</h3>
      </div>
      <div className="divide-y divide-gray-50">
        {recommendations.map((rec, i) => (
          <div key={i} className="flex gap-4 p-5 items-start">
            <span className="text-xs text-gray-300 font-mono mt-0.5 min-w-[24px]">
              {(i + 1).toString().padStart(2, '0')}
            </span>
            <div className="flex-1">
              <p className="font-medium text-sm text-gray-800 mb-2">{rec.text}</p>
              <div className="flex gap-1.5 flex-wrap">
                <span className={`inline-flex px-2.5 py-0.5 rounded-full text-xs font-semibold ${PRIORITY_BADGE[rec.priority]}`}>
                  {rec.priority}
                </span>
                <span className={`inline-flex px-2.5 py-0.5 rounded-full text-xs font-semibold ${PHASE_BADGE}`}>
                  {rec.phase}
                </span>
                <span className={`inline-flex px-2.5 py-0.5 rounded-full text-xs font-semibold ${DOMAIN_BADGE}`}>
                  {rec.domain}
                </span>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
