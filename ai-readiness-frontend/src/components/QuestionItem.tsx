'use client';
// components/QuestionItem.tsx
import { useState } from 'react';
import type { Question, AnswerValue } from '@/lib/types';

const SCORE_LABELS: Record<number, string> = {
  1: 'Not Present', 2: 'Ad-hoc', 3: 'Defined', 4: 'Managed', 5: 'Optimized',
};

interface Props {
  question: Question;
  answer: AnswerValue | undefined;
  onChange: (answer: AnswerValue) => void;
}

export function QuestionItem({ question, answer, onChange }: Props) {
  const [showComment, setShowComment] = useState(!!(answer?.comment));
  const score = answer?.score;

  return (
    <div className="border border-gray-200 rounded-xl bg-white p-5 focus-within:border-blue-300 transition-colors">
      {/* Meta row */}
      <div className="flex items-center gap-2 mb-2">
        {question.isCritical && (
          <span className="text-xs bg-red-100 text-red-700 px-2 py-0.5 rounded-full font-semibold">⚡ Critical</span>
        )}
        <span className="text-xs text-gray-300 font-mono">W:{question.weight}</span>
      </div>

      {/* Question text */}
      <p className="text-sm font-medium text-gray-800 mb-4 leading-relaxed">{question.text}</p>

      {/* Score pills */}
      <div className="flex gap-1.5 bg-gray-50 p-1.5 rounded-full w-fit mb-1">
        {[1, 2, 3, 4, 5].map((n) => (
          <button key={n} onClick={() => onChange({ ...answer, score: n })}
            className={`w-10 h-10 rounded-full text-sm font-semibold transition-all
              ${score === n
                ? 'bg-blue-600 text-white shadow-sm'
                : 'text-gray-400 hover:bg-blue-50 hover:text-blue-600'}`}>
            {n}
          </button>
        ))}
      </div>
      <div className="flex justify-between text-[10px] text-gray-300 px-2 mb-3" style={{ width: 218 }}>
        <span>Not Present</span>
        <span>Optimized</span>
      </div>

      {/* Comment toggle */}
      <button onClick={() => setShowComment(s => !s)}
        className="text-xs text-blue-500 hover:text-blue-700 flex items-center gap-1 mt-1 transition-colors">
        {showComment ? '▲ Hide notes' : '▼ Add notes / evidence'}
      </button>
      {showComment && (
        <textarea
          className="w-full mt-2 p-3 border border-gray-200 rounded-lg text-sm resize-y min-h-[72px]
            focus:outline-none focus:border-blue-300 text-gray-700 bg-gray-50"
          placeholder="Notes, evidence links, or context..."
          value={answer?.comment || ''}
          onChange={e => onChange({ ...answer, comment: e.target.value })}
        />
      )}
    </div>
  );
}
