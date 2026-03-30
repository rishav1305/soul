// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { FreelanceGate } from './FreelanceGate';

vi.mock('../../lib/api', () => ({
  api: {
    post: vi.fn().mockResolvedValue({ result: 'proposal text' }),
  },
}));

function makeLead(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'React Dashboard',
    company: 'ClientCo',
    type: 'freelance' as const,
    source: 'upwork',
    stage: 'qualified',
    match_score: 75,
    compensation: '$5k',
    contact: '',
    location: 'Remote',
    notes: '',
    url: '',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

describe('FreelanceGate', () => {
  afterEach(() => cleanup());

  it('renders freelance gate container', () => {
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('freelance-gate')).toBeTruthy();
  });

  it('shows lead title', () => {
    render(<FreelanceGate lead={makeLead({ title: 'Node API' })} onAction={vi.fn()} />);
    expect(screen.getByText('Node API')).toBeTruthy();
  });

  it('shows company', () => {
    render(<FreelanceGate lead={makeLead({ company: 'FreelanceCo' })} onAction={vi.fn()} />);
    expect(screen.getByText('FreelanceCo')).toBeTruthy();
  });

  it('shows score badge', () => {
    render(<FreelanceGate lead={makeLead({ match_score: 85 })} onAction={vi.fn()} />);
    expect(screen.getByTestId('freelance-gate-score-badge').textContent).toBe('85%');
  });

  it('renders gig score section', () => {
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('freelance-gate-section-score')).toBeTruthy();
    expect(screen.getByText('Gig Score')).toBeTruthy();
  });

  it('renders proposal section', () => {
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('freelance-gate-section-proposal')).toBeTruthy();
    expect(screen.getByText('Proposal Draft')).toBeTruthy();
  });

  it('renders generate buttons', () => {
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('freelance-gate-generate-score')).toBeTruthy();
    expect(screen.getByTestId('freelance-gate-generate-proposal')).toBeTruthy();
  });

  it('renders footer action buttons', () => {
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('freelance-gate-skip')).toBeTruthy();
    expect(screen.getByTestId('freelance-gate-edit')).toBeTruthy();
    expect(screen.getByTestId('freelance-gate-submit')).toBeTruthy();
  });

  it('calls onAction with skip', () => {
    const onAction = vi.fn();
    render(<FreelanceGate lead={makeLead({ id: 7 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('freelance-gate-skip'));
    expect(onAction).toHaveBeenCalledWith('skip', 7);
  });

  it('calls onAction with submit', () => {
    const onAction = vi.fn();
    render(<FreelanceGate lead={makeLead({ id: 7 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('freelance-gate-submit'));
    expect(onAction).toHaveBeenCalledWith('submit', 7);
  });

  it('shows Submit Proposal button text', () => {
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByText('Submit Proposal')).toBeTruthy();
  });
});
