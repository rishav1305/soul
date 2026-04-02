// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import { JobGate } from './JobGate';

vi.mock('../../lib/api', () => ({
  api: {
    post: vi.fn().mockResolvedValue({ result: 'generated content' }),
  },
}));

function makeLead(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'Senior Engineer',
    company: 'Acme Corp',
    type: 'job' as const,
    source: 'linkedin',
    stage: 'qualified',
    match_score: 80,
    compensation: '200k',
    contact: '',
    location: 'Remote',
    notes: '',
    url: '',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

describe('JobGate', () => {
  afterEach(() => cleanup());

  it('renders job gate container', () => {
    render(<JobGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('job-gate')).toBeTruthy();
  });

  it('shows lead title', () => {
    render(<JobGate lead={makeLead({ title: 'ML Engineer' })} onAction={vi.fn()} />);
    expect(screen.getByText('ML Engineer')).toBeTruthy();
  });

  it('shows company', () => {
    render(<JobGate lead={makeLead({ company: 'BigCo' })} onAction={vi.fn()} />);
    expect(screen.getByText('BigCo')).toBeTruthy();
  });

  it('shows Tier 1 badge for score >= 80', () => {
    render(<JobGate lead={makeLead({ match_score: 85 })} onAction={vi.fn()} />);
    expect(screen.getByTestId('job-gate-tier').textContent).toBe('Tier 1');
  });

  it('shows Tier 2 badge for score 60-79', () => {
    render(<JobGate lead={makeLead({ match_score: 65 })} onAction={vi.fn()} />);
    expect(screen.getByTestId('job-gate-tier').textContent).toBe('Tier 2');
  });

  it('shows Tier 3 badge for score < 60', () => {
    render(<JobGate lead={makeLead({ match_score: 40 })} onAction={vi.fn()} />);
    expect(screen.getByTestId('job-gate-tier').textContent).toBe('Tier 3');
  });

  it('shows match score percentage', () => {
    render(<JobGate lead={makeLead({ match_score: 72 })} onAction={vi.fn()} />);
    expect(screen.getByTestId('job-gate-score').textContent).toBe('72%');
  });

  it('renders all 3 sections', () => {
    render(<JobGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('job-gate-section-resume')).toBeTruthy();
    expect(screen.getByTestId('job-gate-section-cover')).toBeTruthy();
    expect(screen.getByTestId('job-gate-section-outreach')).toBeTruthy();
  });

  it('shows section labels', () => {
    render(<JobGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByText('Resume Diff')).toBeTruthy();
    expect(screen.getByText('Cover Letter')).toBeTruthy();
    expect(screen.getByText('Outreach Draft')).toBeTruthy();
  });

  it('renders generate buttons for all sections', () => {
    render(<JobGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('job-gate-generate-resume')).toBeTruthy();
    expect(screen.getByTestId('job-gate-generate-cover')).toBeTruthy();
    expect(screen.getByTestId('job-gate-generate-outreach')).toBeTruthy();
  });

  it('calls api.post when Generate clicked', async () => {
    const { api } = await import('../../lib/api');
    render(<JobGate lead={makeLead({ id: 42 })} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('job-gate-generate-resume'));
    });
    expect(api.post).toHaveBeenCalledWith('/api/ai/resume-tailor', { lead_id: 42 });
  });

  it('shows generated content', async () => {
    render(<JobGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('job-gate-generate-resume'));
    });
    expect(screen.getByTestId('job-gate-content-resume').textContent).toBe('generated content');
  });

  it('renders footer action buttons', () => {
    render(<JobGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('job-gate-skip')).toBeTruthy();
    expect(screen.getByTestId('job-gate-edit')).toBeTruthy();
    expect(screen.getByTestId('job-gate-approve')).toBeTruthy();
  });

  it('calls onAction with skip', () => {
    const onAction = vi.fn();
    render(<JobGate lead={makeLead({ id: 5 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('job-gate-skip'));
    expect(onAction).toHaveBeenCalledWith('skip', 5);
  });

  it('calls onAction with edit', () => {
    const onAction = vi.fn();
    render(<JobGate lead={makeLead({ id: 5 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('job-gate-edit'));
    expect(onAction).toHaveBeenCalledWith('edit', 5);
  });

  it('calls onAction with approve', () => {
    const onAction = vi.fn();
    render(<JobGate lead={makeLead({ id: 5 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('job-gate-approve'));
    expect(onAction).toHaveBeenCalledWith('approve', 5);
  });
});
