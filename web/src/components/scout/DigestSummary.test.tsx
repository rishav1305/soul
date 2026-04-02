// @vitest-environment happy-dom
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { DigestSummary } from './DigestSummary';

function makeLead(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'Engineer',
    company: 'Acme',
    stage: 'discovered',
    source: 'linkedin',
    match_score: 80,
    pipeline: 'job',
    priority: 'medium',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

function makeAnalytics(overrides: Record<string, unknown> = {}) {
  return {
    total_leads: 10,
    active_leads: 5,
    by_stage: { discovered: 3, applied: 2 },
    by_source: { linkedin: 6, github: 3, naukri: 1 },
    ...overrides,
  };
}

describe('DigestSummary', () => {
  afterEach(() => cleanup());

  it('renders digest summary container', () => {
    render(<DigestSummary leads={[]} analytics={null} />);
    expect(screen.getByTestId('digest-summary')).toBeTruthy();
  });

  it('shows Weekly Digest title', () => {
    render(<DigestSummary leads={[]} analytics={null} />);
    expect(screen.getByText('Weekly Digest')).toBeTruthy();
  });

  it('shows new leads count for recent leads', () => {
    const recentLead = makeLead({ created_at: new Date().toISOString() });
    render(<DigestSummary leads={[recentLead]} analytics={null} />);
    const newLeads = screen.getByTestId('digest-new-leads');
    expect(newLeads.textContent).toContain('1');
  });

  it('shows 0 new leads for old leads', () => {
    const oldDate = new Date(Date.now() - 14 * 24 * 60 * 60 * 1000).toISOString();
    const oldLead = makeLead({ created_at: oldDate });
    render(<DigestSummary leads={[oldLead]} analytics={null} />);
    const newLeads = screen.getByTestId('digest-new-leads');
    expect(newLeads.textContent).toContain('0');
  });

  it('shows active leads from analytics', () => {
    render(<DigestSummary leads={[]} analytics={makeAnalytics({ active_leads: 7 })} />);
    const active = screen.getByTestId('digest-active');
    expect(active.textContent).toContain('7');
  });

  it('shows follow-ups due for stale leads', () => {
    const staleDate = new Date(Date.now() - 10 * 24 * 60 * 60 * 1000).toISOString();
    const staleLead = makeLead({ stage: 'applied', updated_at: staleDate });
    render(<DigestSummary leads={[staleLead]} analytics={null} />);
    const followups = screen.getByTestId('digest-followups');
    expect(followups.textContent).toContain('1');
  });

  it('excludes closed/rejected from follow-ups', () => {
    const staleDate = new Date(Date.now() - 10 * 24 * 60 * 60 * 1000).toISOString();
    const closedLead = makeLead({ stage: 'closed', updated_at: staleDate });
    render(<DigestSummary leads={[closedLead]} analytics={null} />);
    const followups = screen.getByTestId('digest-followups');
    expect(followups.textContent).toContain('0');
  });

  it('shows total leads from analytics', () => {
    render(<DigestSummary leads={[]} analytics={makeAnalytics({ total_leads: 52 })} />);
    const total = screen.getByTestId('digest-total');
    expect(total.textContent).toContain('52');
  });

  it('shows top sources when analytics available', () => {
    render(<DigestSummary leads={[]} analytics={makeAnalytics()} />);
    expect(screen.getByText('Top Sources')).toBeTruthy();
    expect(screen.getByText('linkedin (6)')).toBeTruthy();
    expect(screen.getByText('github (3)')).toBeTruthy();
  });
});
