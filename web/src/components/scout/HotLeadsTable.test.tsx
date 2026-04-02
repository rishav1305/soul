// @vitest-environment happy-dom
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { HotLeadsTable } from './HotLeadsTable';

function makeLead(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'Go Developer',
    company: 'Acme Corp',
    type: 'job',
    stage: 'discovered',
    match_score: 85,
    ...overrides,
  };
}

describe('HotLeadsTable', () => {
  afterEach(() => cleanup());

  it('renders hot leads container', () => {
    render(<HotLeadsTable leads={[]} />);
    expect(screen.getByTestId('hot-leads-table')).toBeTruthy();
  });

  it('shows Hot Leads title', () => {
    render(<HotLeadsTable leads={[]} />);
    expect(screen.getByText('Hot Leads')).toBeTruthy();
  });

  it('shows empty state when no leads', () => {
    render(<HotLeadsTable leads={[]} />);
    expect(screen.getByText('No scored leads available')).toBeTruthy();
  });

  it('renders lead rows', () => {
    const leads = [makeLead({ id: 1 }), makeLead({ id: 2, title: 'React Dev' })];
    render(<HotLeadsTable leads={leads} />);
    expect(screen.getByTestId('hot-lead-row-1')).toBeTruthy();
    expect(screen.getByTestId('hot-lead-row-2')).toBeTruthy();
  });

  it('shows lead title', () => {
    render(<HotLeadsTable leads={[makeLead({ title: 'Senior Engineer' })]} />);
    expect(screen.getByText('Senior Engineer')).toBeTruthy();
  });

  it('shows company name', () => {
    render(<HotLeadsTable leads={[makeLead({ company: 'Google' })]} />);
    expect(screen.getByText('Google')).toBeTruthy();
  });

  it('shows match score with percentage', () => {
    render(<HotLeadsTable leads={[makeLead({ match_score: 92 })]} />);
    expect(screen.getByText('92%')).toBeTruthy();
  });

  it('shows stage', () => {
    render(<HotLeadsTable leads={[makeLead({ stage: 'interviewing' })]} />);
    expect(screen.getByText('interviewing')).toBeTruthy();
  });

  it('sorts by match score descending', () => {
    const leads = [
      makeLead({ id: 1, match_score: 60 }),
      makeLead({ id: 2, match_score: 95 }),
      makeLead({ id: 3, match_score: 75 }),
    ];
    const { container } = render(<HotLeadsTable leads={leads} />);
    const rows = container.querySelectorAll('[data-testid^="hot-lead-row-"]');
    expect(rows[0].getAttribute('data-testid')).toBe('hot-lead-row-2'); // 95 first
    expect(rows[1].getAttribute('data-testid')).toBe('hot-lead-row-3'); // 75 second
    expect(rows[2].getAttribute('data-testid')).toBe('hot-lead-row-1'); // 60 last
  });

  it('shows type', () => {
    render(<HotLeadsTable leads={[makeLead({ type: 'freelance' })]} />);
    expect(screen.getByText('freelance')).toBeTruthy();
  });
});
