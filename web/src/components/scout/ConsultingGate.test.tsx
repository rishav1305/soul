// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ConsultingGate } from './ConsultingGate';

vi.mock('../../lib/api', () => ({
  api: {
    post: vi.fn().mockResolvedValue({
      company_background: 'A tech company',
      likely_questions: ['Q1', 'Q2'],
      relevant_experience: ['Exp1'],
    }),
  },
}));

function makeLead(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'AI Strategy Consulting',
    company: 'ConsultCo',
    type: 'consulting' as const,
    source: 'referral',
    stage: 'screening',
    match_score: 80,
    compensation: '$300/hr',
    contact: '',
    location: 'Remote',
    notes: 'Need AI strategy review',
    url: '',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

describe('ConsultingGate', () => {
  afterEach(() => cleanup());

  it('renders consulting gate container', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('consulting-gate')).toBeTruthy();
  });

  it('shows company in header', () => {
    render(<ConsultingGate lead={makeLead({ company: 'TechCo' })} onAction={vi.fn()} />);
    expect(screen.getByText('TechCo')).toBeTruthy();
  });

  it('shows notes (or title as fallback)', () => {
    render(<ConsultingGate lead={makeLead({ notes: 'Need ML pipeline' })} onAction={vi.fn()} />);
    expect(screen.getByText('Need ML pipeline')).toBeTruthy();
  });

  it('shows title when notes empty', () => {
    render(<ConsultingGate lead={makeLead({ notes: '', title: 'AI Review' })} onAction={vi.fn()} />);
    expect(screen.getByText('AI Review')).toBeTruthy();
  });

  it('shows stage badge', () => {
    render(<ConsultingGate lead={makeLead({ stage: 'interviewing' })} onAction={vi.fn()} />);
    expect(screen.getByText('interviewing')).toBeTruthy();
  });

  it('renders all 4 section toggles', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('consulting-section-call-prep')).toBeTruthy();
    expect(screen.getByTestId('consulting-section-advisory')).toBeTruthy();
    expect(screen.getByTestId('consulting-section-project')).toBeTruthy();
    expect(screen.getByTestId('consulting-section-upsell')).toBeTruthy();
  });

  it('shows section labels', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByText('Call Prep')).toBeTruthy();
    expect(screen.getByText('Advisory Proposal')).toBeTruthy();
    expect(screen.getByText('Project Proposal')).toBeTruthy();
    expect(screen.getByText('Upsell Evaluation')).toBeTruthy();
  });

  it('call-prep section expanded by default', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('consulting-generate-call-prep-btn')).toBeTruthy();
  });

  it('collapses section on toggle click', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('consulting-section-call-prep'));
    expect(screen.queryByTestId('consulting-generate-call-prep-btn')).toBeNull();
  });

  it('expands collapsed section on click', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('consulting-section-advisory'));
    expect(screen.getByTestId('consulting-generate-advisory-btn')).toBeTruthy();
  });

  it('renders footer action buttons', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('consulting-send-proposal-btn')).toBeTruthy();
    expect(screen.getByTestId('consulting-follow-up-btn')).toBeTruthy();
    expect(screen.getByTestId('consulting-skip-btn')).toBeTruthy();
  });

  it('calls onAction with send-proposal', () => {
    const onAction = vi.fn();
    render(<ConsultingGate lead={makeLead({ id: 15 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('consulting-send-proposal-btn'));
    expect(onAction).toHaveBeenCalledWith('send-proposal', 15);
  });

  it('calls onAction with follow-up', () => {
    const onAction = vi.fn();
    render(<ConsultingGate lead={makeLead({ id: 15 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('consulting-follow-up-btn'));
    expect(onAction).toHaveBeenCalledWith('follow-up', 15);
  });

  it('calls onAction with skip', () => {
    const onAction = vi.fn();
    render(<ConsultingGate lead={makeLead({ id: 15 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('consulting-skip-btn'));
    expect(onAction).toHaveBeenCalledWith('skip', 15);
  });

  it('shows button labels', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByText('Send Proposal')).toBeTruthy();
    expect(screen.getByText('Skip')).toBeTruthy();
  });
});
