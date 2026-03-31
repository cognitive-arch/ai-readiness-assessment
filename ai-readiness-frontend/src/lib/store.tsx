'use client';
// lib/store.tsx
// Assessment state management using React Context + localStorage.
// On mount, fetches the question bank from the backend API (with local JSON fallback).
// Persists assessmentId, answers, and result across page reloads.

import React, {
  createContext, useContext, useReducer, useEffect, useCallback, ReactNode, useState,
} from 'react';
import type { AssessmentState, Answers, AnswerValue, AssessmentResult, QuestionBank } from './types';
import { api } from './api';
import localBank from '../../public/question-bank-v1.json';

const STORAGE_KEY = 'ai_assessment_v2';

// ─────────────────────────────────────────────
// Local storage helpers
// ─────────────────────────────────────────────

function loadFromStorage(): AssessmentState {
  if (typeof window === 'undefined') return { answers: {} };
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    return raw ? JSON.parse(raw) : { answers: {} };
  } catch {
    return { answers: {} };
  }
}

function saveToStorage(state: AssessmentState) {
  if (typeof window === 'undefined') return;
  try {
    // Don't persist the live question bank — always re-fetch from API on load
    const { questionBank: _, ...persistable } = state;
    localStorage.setItem(STORAGE_KEY, JSON.stringify(persistable));
  } catch {}
}

// ─────────────────────────────────────────────
// Reducer
// ─────────────────────────────────────────────

type Action =
  | { type: 'SET_ANSWER'; questionId: string; answer: AnswerValue }
  | { type: 'SET_RESULT'; result: AssessmentResult }
  | { type: 'SET_ASSESSMENT_ID'; id: string }
  | { type: 'SET_QUESTION_BANK'; bank: QuestionBank }
  | { type: 'RESET' };

function reducer(state: AssessmentState, action: Action): AssessmentState {
  switch (action.type) {
    case 'SET_ANSWER':
      return { ...state, answers: { ...state.answers, [action.questionId]: action.answer } };
    case 'SET_RESULT':
      return { ...state, result: action.result };
    case 'SET_ASSESSMENT_ID':
      return { ...state, assessmentId: action.id };
    case 'SET_QUESTION_BANK':
      return { ...state, questionBank: action.bank };
    case 'RESET':
      // Keep question bank on reset — no need to re-fetch
      return { answers: {}, questionBank: state.questionBank };
    default:
      return state;
  }
}

// ─────────────────────────────────────────────
// Context
// ─────────────────────────────────────────────

interface StoreContext {
  state: AssessmentState;
  /** The live question bank (API-sourced or local fallback). Always non-null after mount. */
  questionBank: QuestionBank;
  bankLoading: boolean;
  backendAvailable: boolean;
  setAnswer: (questionId: string, answer: AnswerValue) => void;
  setResult: (result: AssessmentResult) => void;
  setAssessmentId: (id: string) => void;
  reset: () => void;
  /** Ensure an assessmentId exists on the backend, creating one if needed. */
  ensureAssessmentId: () => Promise<string>;
}

const Ctx = createContext<StoreContext | null>(null);

// ─────────────────────────────────────────────
// Provider
// ─────────────────────────────────────────────

export function AssessmentProvider({ children }: { children: ReactNode }) {
  const [state, dispatch] = useReducer(reducer, undefined, loadFromStorage);
  const [bankLoading, setBankLoading] = useState(true);
  const [backendAvailable, setBackendAvailable] = useState(false);

  // Persist state (excluding questionBank) whenever it changes
  useEffect(() => { saveToStorage(state); }, [state]);

  // On mount: check backend availability and fetch question bank
  useEffect(() => {
    let cancelled = false;

    async function init() {
      const available = await api.isAvailable();
      if (cancelled) return;
      setBackendAvailable(available);

      if (available) {
        const bank = await api.getQuestions();
        if (!cancelled && bank && bank.questions.length > 0) {
          dispatch({ type: 'SET_QUESTION_BANK', bank });
        }
      }
      // Whether API succeeded or not, we're done loading
      if (!cancelled) setBankLoading(false);
    }

    init();
    return () => { cancelled = true; };
  }, []);

  // Dispatch helpers
  const setAnswer = useCallback((questionId: string, answer: AnswerValue) =>
    dispatch({ type: 'SET_ANSWER', questionId, answer }), []);

  const setResult = useCallback((result: AssessmentResult) =>
    dispatch({ type: 'SET_RESULT', result }), []);

  const setAssessmentId = useCallback((id: string) =>
    dispatch({ type: 'SET_ASSESSMENT_ID', id }), []);

  const reset = useCallback(() => {
    dispatch({ type: 'RESET' });
    if (typeof window !== 'undefined') localStorage.removeItem(STORAGE_KEY);
  }, []);

  // Ensure a backend assessment exists; create one if not
  const ensureAssessmentId = useCallback(async (): Promise<string> => {
    if (state.assessmentId) return state.assessmentId;
    const id = await api.createAssessment();
    dispatch({ type: 'SET_ASSESSMENT_ID', id });
    return id;
  }, [state.assessmentId]);

  // Active question bank: API version if loaded, otherwise bundled local JSON
  const questionBank: QuestionBank = state.questionBank ?? (localBank as QuestionBank);

  return (
    <Ctx.Provider value={{
      state,
      questionBank,
      bankLoading,
      backendAvailable,
      setAnswer,
      setResult,
      setAssessmentId,
      reset,
      ensureAssessmentId,
    }}>
      {children}
    </Ctx.Provider>
  );
}

export function useAssessment() {
  const ctx = useContext(Ctx);
  if (!ctx) throw new Error('useAssessment must be used within AssessmentProvider');
  return ctx;
}
