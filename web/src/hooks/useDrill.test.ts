// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act } from '@testing-library/react';
import { useDrill } from './useDrill';

const mockPost = vi.fn();
vi.mock('../lib/api', () => ({
  api: { post: (...args: unknown[]) => mockPost(...args) },
}));

vi.mock('../lib/telemetry', () => ({
  reportError: vi.fn(),
  reportUsage: vi.fn(),
}));

function makeQuestion(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    question: 'What is a binary tree?',
    options: ['A', 'B', 'C', 'D'],
    topic_id: 10,
    difficulty: 'medium',
    ...overrides,
  };
}

function makeEvaluation(overrides: Record<string, unknown> = {}) {
  return {
    correct: true,
    explanation: 'Good answer',
    score: 1,
    ...overrides,
  };
}

describe('useDrill', () => {
  beforeEach(() => {
    mockPost.mockReset();
  });
  afterEach(() => cleanup());

  it('returns initial state', () => {
    const { result } = renderHook(() => useDrill());
    expect(result.current.question).toBeNull();
    expect(result.current.evaluation).toBeNull();
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeNull();
    expect(result.current.answered).toBe(0);
    expect(result.current.correct).toBe(0);
  });

  it('startDrill sets question from API', async () => {
    const q = makeQuestion();
    mockPost.mockResolvedValue(q);
    const { result } = renderHook(() => useDrill());

    await act(async () => {
      await result.current.startDrill(10);
    });

    expect(mockPost).toHaveBeenCalledWith('/api/tutor/drill/start', { topic_id: 10 });
    expect(result.current.question).toEqual(q);
    expect(result.current.loading).toBe(false);
    expect(result.current.error).toBeNull();
  });

  it('startDrill sets error on failure', async () => {
    mockPost.mockRejectedValue(new Error('Network error'));
    const { result } = renderHook(() => useDrill());

    await act(async () => {
      await result.current.startDrill(10);
    });

    expect(result.current.error).toBe('Network error');
    expect(result.current.question).toBeNull();
    expect(result.current.loading).toBe(false);
  });

  it('startDrill handles non-Error rejection', async () => {
    mockPost.mockRejectedValue('string error');
    const { result } = renderHook(() => useDrill());

    await act(async () => {
      await result.current.startDrill(5);
    });

    expect(result.current.error).toBe('string error');
  });

  it('submitAnswer sets evaluation and increments counters for correct', async () => {
    const evalResult = makeEvaluation({ correct: true });
    mockPost.mockResolvedValue(evalResult);
    const { result } = renderHook(() => useDrill());

    await act(async () => {
      await result.current.submitAnswer(1, 'Binary tree is...');
    });

    expect(mockPost).toHaveBeenCalledWith('/api/tutor/drill/answer', { question_id: 1, answer: 'Binary tree is...' });
    expect(result.current.evaluation).toEqual(evalResult);
    expect(result.current.answered).toBe(1);
    expect(result.current.correct).toBe(1);
  });

  it('submitAnswer increments answered but not correct for wrong', async () => {
    const evalResult = makeEvaluation({ correct: false });
    mockPost.mockResolvedValue(evalResult);
    const { result } = renderHook(() => useDrill());

    await act(async () => {
      await result.current.submitAnswer(1, 'wrong');
    });

    expect(result.current.answered).toBe(1);
    expect(result.current.correct).toBe(0);
  });

  it('submitAnswer sets error on failure', async () => {
    mockPost.mockRejectedValue(new Error('Server error'));
    const { result } = renderHook(() => useDrill());

    await act(async () => {
      await result.current.submitAnswer(1, 'answer');
    });

    expect(result.current.error).toBe('Server error');
    expect(result.current.evaluation).toBeNull();
  });

  it('nextQuestion clears evaluation and calls startDrill', async () => {
    const q1 = makeQuestion({ id: 1 });
    const q2 = makeQuestion({ id: 2, question: 'What is a hash map?' });
    mockPost.mockResolvedValueOnce(q1);

    const { result } = renderHook(() => useDrill());

    // Start initial drill
    await act(async () => {
      await result.current.startDrill(10);
    });
    expect(result.current.question).toEqual(q1);

    // Set evaluation via submitAnswer
    mockPost.mockResolvedValueOnce(makeEvaluation());
    await act(async () => {
      await result.current.submitAnswer(1, 'answer');
    });
    expect(result.current.evaluation).not.toBeNull();

    // Next question clears evaluation
    mockPost.mockResolvedValueOnce(q2);
    await act(async () => {
      await result.current.nextQuestion(10);
    });
    expect(result.current.question).toEqual(q2);
    expect(result.current.loading).toBe(false);
  });

  it('tracks cumulative answered/correct across multiple submits', async () => {
    const { result } = renderHook(() => useDrill());

    mockPost.mockResolvedValueOnce(makeEvaluation({ correct: true }));
    await act(async () => {
      await result.current.submitAnswer(1, 'a');
    });

    mockPost.mockResolvedValueOnce(makeEvaluation({ correct: false }));
    await act(async () => {
      await result.current.submitAnswer(2, 'b');
    });

    mockPost.mockResolvedValueOnce(makeEvaluation({ correct: true }));
    await act(async () => {
      await result.current.submitAnswer(3, 'c');
    });

    expect(result.current.answered).toBe(3);
    expect(result.current.correct).toBe(2);
  });

  it('startDrill clears previous evaluation', async () => {
    const { result } = renderHook(() => useDrill());

    // Submit an answer first
    mockPost.mockResolvedValueOnce(makeEvaluation());
    await act(async () => {
      await result.current.submitAnswer(1, 'a');
    });
    expect(result.current.evaluation).not.toBeNull();

    // startDrill should clear evaluation
    mockPost.mockResolvedValueOnce(makeQuestion());
    await act(async () => {
      await result.current.startDrill(10);
    });
    expect(result.current.evaluation).toBeNull();
  });

  it('startDrill clears error from previous failure', async () => {
    const { result } = renderHook(() => useDrill());

    // First: error
    mockPost.mockRejectedValueOnce(new Error('Temp error'));
    await act(async () => {
      await result.current.startDrill(10);
    });
    expect(result.current.error).toBe('Temp error');

    // Second: success, error should clear
    mockPost.mockResolvedValueOnce(makeQuestion());
    await act(async () => {
      await result.current.startDrill(10);
    });
    expect(result.current.error).toBeNull();
  });

  it('submitAnswer handles non-Error rejection', async () => {
    mockPost.mockRejectedValue('submit string error');
    const { result } = renderHook(() => useDrill());

    await act(async () => {
      await result.current.submitAnswer(1, 'answer');
    });

    expect(result.current.error).toBe('submit string error');
  });

  it('submitAnswer clears error from previous failure', async () => {
    const { result } = renderHook(() => useDrill());

    // First submit fails
    mockPost.mockRejectedValueOnce(new Error('first error'));
    await act(async () => {
      await result.current.submitAnswer(1, 'a');
    });
    expect(result.current.error).toBe('first error');

    // Second submit succeeds — error clears
    mockPost.mockResolvedValueOnce(makeEvaluation());
    await act(async () => {
      await result.current.submitAnswer(2, 'b');
    });
    expect(result.current.error).toBeNull();
  });

  it('loading is true during startDrill in-flight', async () => {
    let resolvePost: ((v: unknown) => void) | null = null;
    mockPost.mockImplementationOnce(() => new Promise(r => { resolvePost = r; }));

    const { result } = renderHook(() => useDrill());

    let drillPromise: Promise<void>;
    act(() => {
      drillPromise = result.current.startDrill(10);
    });

    expect(result.current.loading).toBe(true);

    await act(async () => {
      resolvePost!(makeQuestion());
      await drillPromise!;
    });
    expect(result.current.loading).toBe(false);
  });

  it('loading is true during submitAnswer in-flight', async () => {
    let resolvePost: ((v: unknown) => void) | null = null;
    mockPost.mockImplementationOnce(() => new Promise(r => { resolvePost = r; }));

    const { result } = renderHook(() => useDrill());

    let answerPromise: Promise<void>;
    act(() => {
      answerPromise = result.current.submitAnswer(1, 'test');
    });

    expect(result.current.loading).toBe(true);

    await act(async () => {
      resolvePost!(makeEvaluation());
      await answerPromise!;
    });
    expect(result.current.loading).toBe(false);
  });

  it('nextQuestion propagates error from startDrill', async () => {
    mockPost.mockRejectedValue(new Error('next error'));
    const { result } = renderHook(() => useDrill());

    await act(async () => {
      await result.current.nextQuestion(10);
    });

    expect(result.current.error).toBe('next error');
    expect(result.current.evaluation).toBeNull();
  });
});
