'use client';
// components/RadarChart.tsx
import {
  RadarChart as ReRadarChart, Radar, PolarGrid, PolarAngleAxis, PolarRadiusAxis,
  Tooltip, ResponsiveContainer,
} from 'recharts';
import type { DomainScores } from '@/lib/types';

interface Props {
  domainScores: DomainScores;
}

export function RadarChart({ domainScores }: Props) {
  const data = Object.entries(domainScores).map(([domain, ds]) => ({
    domain: domain.substring(0, 4),
    fullName: domain,
    score: Math.round(ds.score),
  }));

  return (
    <div className="bg-white border border-gray-200 rounded-xl shadow-sm">
      <div className="px-6 py-4 border-b border-gray-100">
        <h3 className="font-semibold text-gray-800">Domain Radar</h3>
      </div>
      <div className="p-4" style={{ height: 300 }}>
        <ResponsiveContainer width="100%" height="100%">
          <ReRadarChart data={data}>
            <PolarGrid stroke="#f3f4f6" />
            <PolarAngleAxis dataKey="domain" tick={{ fontSize: 12, fill: '#6b7280' }} />
            <PolarRadiusAxis angle={30} domain={[0, 100]} tick={{ fontSize: 10, fill: '#d1d5db' }} />
            <Tooltip
              formatter={(value: number, _: any, props: any) => [`${value}/100`, props.payload.fullName]}
              contentStyle={{ fontSize: 12, borderRadius: 8, border: '1px solid #e5e7eb' }}
            />
            <Radar name="Score" dataKey="score" stroke="#2563eb" fill="#3b82f6" fillOpacity={0.25} />
          </ReRadarChart>
        </ResponsiveContainer>
      </div>
    </div>
  );
}
