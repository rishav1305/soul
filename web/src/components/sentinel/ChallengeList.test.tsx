// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ChallengeList } from './ChallengeList';

function makeChallenge(overrides: Record<string, unknown> = {}) {
  return {
    id: 'ch-1',
    title: 'Basic Prompt Injection',
    category: 'prompt_injection',
    difficulty: 'beginner',
    points: 100,
    completed: false,
    ...overrides,
  };
}

describe('ChallengeList', () => {
  afterEach(() => cleanup());

  it('renders challenge list container', () => {
    render(<ChallengeList challenges={[]} onStart={vi.fn()} />);
    expect(screen.getByTestId('challenge-list')).toBeTruthy();
  });

  it('shows empty state when no challenges match', () => {
    render(<ChallengeList challenges={[]} onStart={vi.fn()} />);
    expect(screen.getByText('No challenges match the current filters.')).toBeTruthy();
  });

  it('renders challenge cards', () => {
    const challenges = [
      makeChallenge({ id: 'ch-1' }),
      makeChallenge({ id: 'ch-2', title: 'Advanced Jailbreak' }),
    ];
    render(<ChallengeList challenges={challenges} onStart={vi.fn()} />);
    expect(screen.getByTestId('challenge-card-ch-1')).toBeTruthy();
    expect(screen.getByTestId('challenge-card-ch-2')).toBeTruthy();
  });

  it('shows challenge title', () => {
    render(<ChallengeList challenges={[makeChallenge({ title: 'SQL Injection Test' })]} onStart={vi.fn()} />);
    expect(screen.getByText('SQL Injection Test')).toBeTruthy();
  });

  it('shows category badge', () => {
    render(<ChallengeList challenges={[makeChallenge({ category: 'evasion' })]} onStart={vi.fn()} />);
    // Text appears in both filter option and card badge; check card contains it
    const card = screen.getByTestId('challenge-card-ch-1');
    expect(card.textContent).toContain('evasion');
  });

  it('shows difficulty badge', () => {
    render(<ChallengeList challenges={[makeChallenge({ difficulty: 'advanced' })]} onStart={vi.fn()} />);
    // Text appears in both filter option and card badge; check card contains it
    const card = screen.getByTestId('challenge-card-ch-1');
    expect(card.textContent).toContain('advanced');
  });

  it('shows points', () => {
    render(<ChallengeList challenges={[makeChallenge({ points: 250 })]} onStart={vi.fn()} />);
    expect(screen.getByText('250 pts')).toBeTruthy();
  });

  it('shows Start for incomplete challenges', () => {
    render(<ChallengeList challenges={[makeChallenge({ completed: false })]} onStart={vi.fn()} />);
    expect(screen.getByText('Start')).toBeTruthy();
  });

  it('shows Completed and checkmark for completed challenges', () => {
    render(<ChallengeList challenges={[makeChallenge({ completed: true })]} onStart={vi.fn()} />);
    expect(screen.getByText('Completed')).toBeTruthy();
    expect(screen.getByTestId('challenge-complete-ch-1')).toBeTruthy();
  });

  it('calls onStart when challenge card clicked', () => {
    const onStart = vi.fn();
    render(<ChallengeList challenges={[makeChallenge({ id: 'ch-5' })]} onStart={onStart} />);
    fireEvent.click(screen.getByTestId('challenge-card-ch-5'));
    expect(onStart).toHaveBeenCalledWith('ch-5');
  });

  it('renders category filter', () => {
    render(<ChallengeList challenges={[makeChallenge()]} onStart={vi.fn()} />);
    expect(screen.getByTestId('filter-category')).toBeTruthy();
  });

  it('renders difficulty filter', () => {
    render(<ChallengeList challenges={[makeChallenge()]} onStart={vi.fn()} />);
    expect(screen.getByTestId('filter-difficulty')).toBeTruthy();
  });

  it('filters by category', () => {
    const challenges = [
      makeChallenge({ id: 'ch-1', category: 'evasion', title: 'Evasion One' }),
      makeChallenge({ id: 'ch-2', category: 'jailbreaking', title: 'Jailbreak One' }),
    ];
    render(<ChallengeList challenges={challenges} onStart={vi.fn()} />);
    fireEvent.change(screen.getByTestId('filter-category'), { target: { value: 'evasion' } });
    expect(screen.getByTestId('challenge-card-ch-1')).toBeTruthy();
    expect(screen.queryByTestId('challenge-card-ch-2')).toBeNull();
  });

  it('filters by difficulty', () => {
    const challenges = [
      makeChallenge({ id: 'ch-1', difficulty: 'beginner', title: 'Easy' }),
      makeChallenge({ id: 'ch-2', difficulty: 'advanced', title: 'Hard' }),
    ];
    render(<ChallengeList challenges={challenges} onStart={vi.fn()} />);
    fireEvent.change(screen.getByTestId('filter-difficulty'), { target: { value: 'advanced' } });
    expect(screen.queryByTestId('challenge-card-ch-1')).toBeNull();
    expect(screen.getByTestId('challenge-card-ch-2')).toBeTruthy();
  });
});
