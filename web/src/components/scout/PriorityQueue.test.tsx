// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { PriorityQueue } from './PriorityQueue';

vi.mock('./GateAction', () => ({
  GateAction: ({ actions, onAction }: { actions: string[]; onAction: (a: string) => void }) => (
    <div data-testid="mock-gate-action">
      {actions.map(a => (
        <button key={a} data-testid={`gate-${a}`} onClick={() => onAction(a)}>{a}</button>
      ))}
    </div>
  ),
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
    contact: 'hr@acme.com',
    location: 'Remote',
    notes: '',
    url: 'https://acme.com/jobs/1',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

function daysAgo(n: number): string {
  return new Date(Date.now() - n * 24 * 60 * 60 * 1000).toISOString();
}

describe('PriorityQueue', () => {
  afterEach(() => cleanup());

  it('renders priority queue container', () => {
    render(<PriorityQueue leads={[]} />);
    expect(screen.getByTestId('priority-queue')).toBeTruthy();
  });

  it('shows empty state when no actionable leads', () => {
    render(<PriorityQueue leads={[]} />);
    expect(screen.getByTestId('priority-empty')).toBeTruthy();
    expect(screen.getByText('No pending action items')).toBeTruthy();
  });

  it('shows 0 items in summary when empty', () => {
    render(<PriorityQueue leads={[]} />);
    const summary = screen.getByTestId('priority-summary');
    expect(summary.textContent).toContain('0 items');
  });

  it('creates priority item for high-match lead at action stage', () => {
    const leads = [makeLead({ id: 1, match_score: 80, stage: 'qualified' })];
    render(<PriorityQueue leads={leads} />);
    expect(screen.getByTestId('priority-item-1')).toBeTruthy();
  });

  it('shows lead title in priority item', () => {
    const leads = [makeLead({ title: 'ML Engineer', match_score: 80, stage: 'qualified' })];
    render(<PriorityQueue leads={leads} />);
    expect(screen.getByText('ML Engineer')).toBeTruthy();
  });

  it('shows company in priority item', () => {
    const leads = [makeLead({ company: 'BigCo', match_score: 80, stage: 'qualified' })];
    render(<PriorityQueue leads={leads} />);
    expect(screen.getByText('BigCo')).toBeTruthy();
  });

  it('shows pipeline badge', () => {
    const leads = [makeLead({ type: 'freelance', match_score: 80, stage: 'qualified' })];
    render(<PriorityQueue leads={leads} />);
    expect(screen.getByText('freelance')).toBeTruthy();
  });

  it('assigns high urgency for 85+ match score', () => {
    const leads = [makeLead({ match_score: 90, stage: 'qualified' })];
    render(<PriorityQueue leads={leads} />);
    expect(screen.getByText('high')).toBeTruthy();
  });

  it('assigns medium urgency for 70-84 match score', () => {
    const leads = [makeLead({ match_score: 75, stage: 'qualified' })];
    render(<PriorityQueue leads={leads} />);
    expect(screen.getByText('medium')).toBeTruthy();
  });

  it('shows prepare type for qualified stage', () => {
    const leads = [makeLead({ match_score: 80, stage: 'qualified' })];
    render(<PriorityQueue leads={leads} />);
    const item = screen.getByTestId('priority-item-1');
    expect(item.textContent).toContain('prepare');
  });

  it('shows approve type for proposal-ready stage', () => {
    const leads = [makeLead({ match_score: 80, stage: 'proposal-ready' })];
    render(<PriorityQueue leads={leads} />);
    const item = screen.getByTestId('priority-item-1');
    expect(item.textContent).toContain('approve');
  });

  it('shows review type for screening stage', () => {
    const leads = [makeLead({ match_score: 80, stage: 'screening' })];
    render(<PriorityQueue leads={leads} />);
    const item = screen.getByTestId('priority-item-1');
    expect(item.textContent).toContain('review');
  });

  it('creates stale item for 7+ day old lead', () => {
    const leads = [makeLead({ match_score: 40, stage: 'new', updated_at: daysAgo(10) })];
    render(<PriorityQueue leads={leads} />);
    const item = screen.getByTestId('priority-item-1');
    expect(item.textContent).toContain('10 days');
  });

  it('assigns high urgency for 14+ day stale leads', () => {
    const leads = [makeLead({ match_score: 40, stage: 'new', updated_at: daysAgo(15) })];
    render(<PriorityQueue leads={leads} />);
    expect(screen.getByText('high')).toBeTruthy();
  });

  it('creates followup item for medium-score 3+ day lead', () => {
    const leads = [makeLead({ match_score: 50, stage: 'new', updated_at: daysAgo(4) })];
    render(<PriorityQueue leads={leads} />);
    const item = screen.getByTestId('priority-item-1');
    expect(item.textContent).toContain('follow-up');
  });

  it('skips closed leads', () => {
    const leads = [makeLead({ match_score: 80, stage: 'closed' })];
    render(<PriorityQueue leads={leads} />);
    expect(screen.getByTestId('priority-empty')).toBeTruthy();
  });

  it('skips rejected leads', () => {
    const leads = [makeLead({ match_score: 80, stage: 'rejected' })];
    render(<PriorityQueue leads={leads} />);
    expect(screen.getByTestId('priority-empty')).toBeTruthy();
  });

  it('sorts items by urgency (high first)', () => {
    const leads = [
      makeLead({ id: 1, match_score: 50, stage: 'new', updated_at: daysAgo(4) }), // low urgency (followup)
      makeLead({ id: 2, match_score: 90, stage: 'qualified' }),                    // high urgency
    ];
    const { container } = render(<PriorityQueue leads={leads} />);
    const items = container.querySelectorAll('[data-testid^="priority-item-"]');
    expect(items[0].textContent).toContain('high');
  });

  it('shows urgency counts in summary', () => {
    const leads = [
      makeLead({ id: 1, match_score: 90, stage: 'qualified' }),  // high
      makeLead({ id: 2, match_score: 75, stage: 'screening' }),  // medium
    ];
    render(<PriorityQueue leads={leads} />);
    const summary = screen.getByTestId('priority-summary');
    expect(summary.textContent).toContain('2 items');
    expect(summary.textContent).toContain('1 high');
    expect(summary.textContent).toContain('1 medium');
  });

  it('renders GateAction for each item', () => {
    const leads = [makeLead({ match_score: 80, stage: 'qualified' })];
    render(<PriorityQueue leads={leads} />);
    expect(screen.getByTestId('mock-gate-action')).toBeTruthy();
  });

  it('calls onAction with leadId and action', () => {
    const onAction = vi.fn();
    const leads = [makeLead({ id: 42, match_score: 80, stage: 'qualified' })];
    render(<PriorityQueue leads={leads} onAction={onAction} />);
    screen.getByTestId('gate-approve').click();
    expect(onAction).toHaveBeenCalledWith(42, 'approve');
  });

  it('shows description with match score', () => {
    const leads = [makeLead({ match_score: 80, stage: 'qualified' })];
    render(<PriorityQueue leads={leads} />);
    expect(screen.getByText('80% match — qualified stage, needs prepare')).toBeTruthy();
  });
});
