// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ActionsView } from './ActionsView';

function daysAgo(n: number): string {
  return new Date(Date.now() - n * 24 * 60 * 60 * 1000).toISOString();
}

function makeLead(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'Engineer',
    company: 'Acme',
    type: 'job' as const,
    source: 'linkedin',
    stage: 'screening',
    match_score: 70,
    compensation: '',
    contact: '',
    location: '',
    notes: '',
    url: '',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

function makeOpt(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    type: 'stage_change',
    field: 'stage',
    current: 'screening',
    suggested: 'interviewing',
    reason: 'Ready for interview',
    status: 'pending' as const,
    ...overrides,
  };
}

describe('ActionsView', () => {
  afterEach(() => cleanup());

  it('renders actions view container', () => {
    render(<ActionsView leads={[]} optimizations={[]} onApproveOptimization={vi.fn()} onRejectOptimization={vi.fn()} />);
    expect(screen.getByTestId('actions-view')).toBeTruthy();
  });

  it('renders followups section', () => {
    render(<ActionsView leads={[]} optimizations={[]} onApproveOptimization={vi.fn()} onRejectOptimization={vi.fn()} />);
    expect(screen.getByTestId('actions-followups')).toBeTruthy();
  });

  it('shows no follow-ups when all recent', () => {
    const leads = [makeLead({ updated_at: new Date().toISOString() })];
    render(<ActionsView leads={leads} optimizations={[]} onApproveOptimization={vi.fn()} onRejectOptimization={vi.fn()} />);
    expect(screen.getByText('No follow-ups due')).toBeTruthy();
  });

  it('shows follow-up rows for 7+ day old leads', () => {
    const leads = [makeLead({ id: 1, updated_at: daysAgo(10) })];
    render(<ActionsView leads={leads} optimizations={[]} onApproveOptimization={vi.fn()} onRejectOptimization={vi.fn()} />);
    expect(screen.getByTestId('followup-row-1')).toBeTruthy();
  });

  it('renders stale section', () => {
    render(<ActionsView leads={[]} optimizations={[]} onApproveOptimization={vi.fn()} onRejectOptimization={vi.fn()} />);
    expect(screen.getByTestId('actions-stale')).toBeTruthy();
  });

  it('shows stale leads for 14+ day old leads', () => {
    const leads = [makeLead({ id: 2, updated_at: daysAgo(15) })];
    render(<ActionsView leads={leads} optimizations={[]} onApproveOptimization={vi.fn()} onRejectOptimization={vi.fn()} />);
    expect(screen.getByTestId('stale-lead-2')).toBeTruthy();
  });

  it('excludes closed/rejected from follow-ups and stale', () => {
    const leads = [makeLead({ stage: 'closed', updated_at: daysAgo(30) })];
    render(<ActionsView leads={leads} optimizations={[]} onApproveOptimization={vi.fn()} onRejectOptimization={vi.fn()} />);
    expect(screen.getByText('No follow-ups due')).toBeTruthy();
    expect(screen.getByText('No stale leads')).toBeTruthy();
  });

  it('renders pipeline gaps section', () => {
    render(<ActionsView leads={[]} optimizations={[]} onApproveOptimization={vi.fn()} onRejectOptimization={vi.fn()} />);
    expect(screen.getByTestId('actions-gaps')).toBeTruthy();
  });

  it('shows gap stages when missing', () => {
    render(<ActionsView leads={[]} optimizations={[]} onApproveOptimization={vi.fn()} onRejectOptimization={vi.fn()} />);
    expect(screen.getByText('discovered')).toBeTruthy();
  });

  it('shows all stages covered when no gaps', () => {
    const leads = [
      makeLead({ stage: 'discovered' }),
      makeLead({ stage: 'applied' }),
      makeLead({ stage: 'screening' }),
      makeLead({ stage: 'interviewing' }),
      makeLead({ stage: 'negotiating' }),
    ];
    render(<ActionsView leads={leads} optimizations={[]} onApproveOptimization={vi.fn()} onRejectOptimization={vi.fn()} />);
    expect(screen.getByText('All stages covered')).toBeTruthy();
  });

  it('renders source effectiveness section', () => {
    render(<ActionsView leads={[]} optimizations={[]} onApproveOptimization={vi.fn()} onRejectOptimization={vi.fn()} />);
    expect(screen.getByTestId('actions-source-effectiveness')).toBeTruthy();
  });

  it('shows pending optimizations', () => {
    const opts = [makeOpt({ id: 5 })];
    render(<ActionsView leads={[]} optimizations={opts} onApproveOptimization={vi.fn()} onRejectOptimization={vi.fn()} />);
    expect(screen.getByTestId('actions-optimizations')).toBeTruthy();
    expect(screen.getByTestId('optimization-5')).toBeTruthy();
  });

  it('calls onApproveOptimization', () => {
    const onApprove = vi.fn();
    const opts = [makeOpt({ id: 5 })];
    render(<ActionsView leads={[]} optimizations={opts} onApproveOptimization={onApprove} onRejectOptimization={vi.fn()} />);
    fireEvent.click(screen.getByTestId('approve-opt-5'));
    expect(onApprove).toHaveBeenCalledWith(5);
  });

  it('calls onRejectOptimization', () => {
    const onReject = vi.fn();
    const opts = [makeOpt({ id: 5 })];
    render(<ActionsView leads={[]} optimizations={opts} onApproveOptimization={vi.fn()} onRejectOptimization={onReject} />);
    fireEvent.click(screen.getByTestId('reject-opt-5'));
    expect(onReject).toHaveBeenCalledWith(5);
  });

  it('hides optimizations section when none pending', () => {
    const opts = [makeOpt({ status: 'approved' })];
    render(<ActionsView leads={[]} optimizations={opts} onApproveOptimization={vi.fn()} onRejectOptimization={vi.fn()} />);
    expect(screen.queryByTestId('actions-optimizations')).toBeNull();
  });
});
