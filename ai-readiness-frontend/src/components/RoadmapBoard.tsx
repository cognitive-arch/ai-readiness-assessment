'use client';
// components/RoadmapBoard.tsx
import type { Recommendation } from '@/lib/types';

const PHASE_CONFIG = {
  'Phase 1': {
    color: '#ef4444', bg: '#fef2f2', timeframe: '0–3 months',
    label: 'Immediate Actions',
  },
  'Phase 2': {
    color: '#f59e0b', bg: '#fffbeb', timeframe: '3–9 months',
    label: 'Short-term Improvements',
  },
  'Phase 3': {
    color: '#22c55e', bg: '#f0fdf4', timeframe: '9–18 months',
    label: 'Strategic Foundations',
  },
};

const PRIORITY_DOT: Record<string, string> = {
  Critical: 'bg-red-500',
  High:     'bg-yellow-400',
  Medium:   'bg-green-500',
};

interface Props {
  recommendations: Recommendation[];
}

export function RoadmapBoard({ recommendations }: Props) {
  const phases = Object.keys(PHASE_CONFIG) as (keyof typeof PHASE_CONFIG)[];
  const groups = phases.reduce<Record<string, Recommendation[]>>((acc, p) => {
    acc[p] = recommendations.filter(r => r.phase === p);
    return acc;
  }, {});

  return (
    <div className="space-y-6">
      {phases.map(phase => {
        const cfg = PHASE_CONFIG[phase];
        const items = groups[phase];
        if (!items.length) return null;
        return (
          <div key={phase}>
            <div className="flex items-center gap-3 mb-3 pb-2 border-b-2" style={{ borderColor: cfg.color }}>
              <span className="font-bold text-sm uppercase tracking-wider" style={{ color: cfg.color }}>{phase}</span>
              <span className="text-gray-400 text-sm">{cfg.timeframe}</span>
              <span className="text-xs font-semibold px-2.5 py-1 rounded-full ml-auto" style={{ background: cfg.bg, color: cfg.color }}>
                {cfg.label}
              </span>
            </div>
            <div className="space-y-2">
              {items.map((item, i) => (
                <div key={i} className="flex gap-3 items-start bg-white border border-gray-200 rounded-lg p-4 shadow-sm">
                  <div className={`w-2.5 h-2.5 rounded-full flex-shrink-0 mt-1.5 ${PRIORITY_DOT[item.priority]}`} />
                  <div className="flex-1">
                    <p className="text-sm font-medium text-gray-800">{item.text}</p>
                    <div className="flex gap-2 mt-2">
                      <span className={`text-xs font-semibold px-2 py-0.5 rounded-full
                        ${item.priority === 'Critical' ? 'bg-red-100 text-red-700' :
                          item.priority === 'High' ? 'bg-orange-100 text-orange-700' : 'bg-yellow-100 text-yellow-700'}`}>
                        {item.priority}
                      </span>
                      <span className="text-xs font-semibold bg-gray-100 text-gray-600 px-2 py-0.5 rounded-full">
                        {item.domain}
                      </span>
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        );
      })}
    </div>
  );
}
