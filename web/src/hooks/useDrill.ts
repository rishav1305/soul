import { useState, useCallback } from 'react';
import type { TutorQuizQuestion, TutorDrillEvaluation } from '../lib/types';
import { api } from '../lib/api';
import { reportError, reportUsage } from '../lib/telemetry';

interface UseDrillReturn {
  question: TutorQuizQuestion | null;
  evaluation: TutorDrillEvaluation | null;
  loading: boolean;
  error: string | null;
  answered: number;
  correct: number;
  startDrill: (topicId: number) => Promise<void>;
  submitAnswer: (questionId: number, answer: string) => Promise<void>;
  nextQuestion: (topicId: number) => Promise<void>;
}

export function useDrill(): UseDrillReturn {
  const [question, setQuestion] = useState<TutorQuizQuestion | null>(null);
  const [evaluation, setEvaluation] = useState<TutorDrillEvaluation | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [answered, setAnswered] = useState(0);
  const [correct, setCorrect] = useState(0);

  const startDrill = useCallback(async (topicId: number) => {
    setLoading(true);
    setError(null);
    setEvaluation(null);
    try {
      const data = await api.post<TutorQuizQuestion>('/api/tutor/drill/start', { topic_id: topicId });
      setQuestion(data);
      reportUsage('drill.start', { topicId });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useDrill.start', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  const submitAnswer = useCallback(async (questionId: number, answer: string) => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.post<TutorDrillEvaluation>('/api/tutor/drill/answer', { question_id: questionId, answer });
      setEvaluation(data);
      setAnswered(prev => prev + 1);
      if (data.correct) setCorrect(prev => prev + 1);
      reportUsage('drill.answer', { questionId, correct: data.correct });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useDrill.answer', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  const nextQuestion = useCallback(async (topicId: number) => {
    setEvaluation(null);
    await startDrill(topicId);
  }, [startDrill]);

  return { question, evaluation, loading, error, answered, correct, startDrill, submitAnswer, nextQuestion };
}
