// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { ProgressBoard } from './ProgressBoard';

function makeProgress(overrides: Record<string, unknown> = {}) {
  return {
    total_points: 450,
    completed: 7,
    total_challenges: 14,
    categories: {
      prompt_injection: 200,
      jailbreaking: 150,
      evasion: 100,
    },
    ...overrides,
  };
}

describe('ProgressBoard', () => {
  afterEach(() => cleanup());

  it('renders progress board container', () => {
    render(<ProgressBoard progress={makeProgress()} />);
    expect(screen.getByTestId('progress-board')).toBeTruthy();
  });

  it('shows total points', () => {
    render(<ProgressBoard progress={makeProgress({ total_points: 750 })} />);
    const card = screen.getByTestId('total-points');
    expect(card.textContent).toContain('750');
  });

  it('shows challenges completed', () => {
    render(<ProgressBoard progress={makeProgress({ completed: 10, total_challenges: 14 })} />);
    const card = screen.getByTestId('challenges-completed');
    expect(card.textContent).toContain('10/14');
  });

  it('shows completion percentage', () => {
    render(<ProgressBoard progress={makeProgress({ completed: 7, total_challenges: 14 })} />);
    const card = screen.getByTestId('completion-pct');
    expect(card.textContent).toContain('50%');
  });

  it('shows 0% when no challenges', () => {
    render(<ProgressBoard progress={makeProgress({ completed: 0, total_challenges: 0 })} />);
    const card = screen.getByTestId('completion-pct');
    expect(card.textContent).toContain('0%');
  });

  it('renders progress bar', () => {
    render(<ProgressBoard progress={makeProgress()} />);
    expect(screen.getByTestId('progress-bar')).toBeTruthy();
  });

  it('shows overall progress label', () => {
    render(<ProgressBoard progress={makeProgress()} />);
    expect(screen.getByText('Overall Progress')).toBeTruthy();
  });

  it('shows Category Breakdown title', () => {
    render(<ProgressBoard progress={makeProgress()} />);
    expect(screen.getByText('Category Breakdown')).toBeTruthy();
  });

  it('renders category cards', () => {
    render(<ProgressBoard progress={makeProgress()} />);
    expect(screen.getByTestId('category-prompt_injection')).toBeTruthy();
    expect(screen.getByTestId('category-jailbreaking')).toBeTruthy();
    expect(screen.getByTestId('category-evasion')).toBeTruthy();
  });

  it('shows category points', () => {
    render(<ProgressBoard progress={makeProgress()} />);
    expect(screen.getByText('200 pts')).toBeTruthy();
    expect(screen.getByText('150 pts')).toBeTruthy();
    expect(screen.getByText('100 pts')).toBeTruthy();
  });

  it('shows empty state when no categories', () => {
    render(<ProgressBoard progress={makeProgress({ categories: {} })} />);
    expect(screen.getByText('No category data available.')).toBeTruthy();
  });
});
