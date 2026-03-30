// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { AnalyticsView } from './AnalyticsView';

function makeAnalytics(overrides: Record<string, unknown> = {}) {
  return {
    total_leads: 50,
    active_leads: 30,
    by_type: { job: 20, freelance: 15, contract: 15 },
    by_source: { linkedin: 25, referral: 15, upwork: 10 },
    by_stage: { discovered: 10, applied: 15, screening: 10, interviewing: 8, negotiating: 5, closed: 2 },
    conversion: [
      { stage: 'discovered', count: 10, rate: 100 },
      { stage: 'applied', count: 8, rate: 80 },
    ],
    weekly_trend: [
      { week: 'W1', count: 5 },
      { week: 'W2', count: 8 },
    ],
    ...overrides,
  } as any;
}

describe('AnalyticsView', () => {
  afterEach(() => cleanup());

  it('renders analytics view container', () => {
    render(<AnalyticsView analytics={makeAnalytics()} />);
    expect(screen.getByTestId('analytics-view')).toBeTruthy();
  });

  it('shows total leads stat', () => {
    render(<AnalyticsView analytics={makeAnalytics({ total_leads: 120 })} />);
    expect(screen.getByTestId('stat-total-leads').textContent).toContain('120');
  });

  it('shows active leads stat', () => {
    render(<AnalyticsView analytics={makeAnalytics({ active_leads: 45 })} />);
    expect(screen.getByTestId('stat-active-leads').textContent).toContain('45');
  });

  it('shows sources count', () => {
    render(<AnalyticsView analytics={makeAnalytics()} />);
    expect(screen.getByTestId('stat-sources').textContent).toContain('3');
  });

  it('shows types count', () => {
    render(<AnalyticsView analytics={makeAnalytics()} />);
    expect(screen.getByTestId('stat-types').textContent).toContain('3');
  });

  it('renders by type section', () => {
    render(<AnalyticsView analytics={makeAnalytics()} />);
    expect(screen.getByTestId('analytics-by-type')).toBeTruthy();
    expect(screen.getByText('By Type')).toBeTruthy();
  });

  it('renders by source section', () => {
    render(<AnalyticsView analytics={makeAnalytics()} />);
    expect(screen.getByTestId('analytics-by-source')).toBeTruthy();
    expect(screen.getByText('By Source')).toBeTruthy();
  });

  it('renders by stage section', () => {
    render(<AnalyticsView analytics={makeAnalytics()} />);
    expect(screen.getByTestId('analytics-by-stage')).toBeTruthy();
    expect(screen.getByText('By Stage')).toBeTruthy();
  });

  it('renders conversion funnel', () => {
    render(<AnalyticsView analytics={makeAnalytics()} />);
    expect(screen.getByTestId('analytics-conversion')).toBeTruthy();
    expect(screen.getByText('Conversion Funnel')).toBeTruthy();
  });

  it('renders weekly trend', () => {
    render(<AnalyticsView analytics={makeAnalytics()} />);
    expect(screen.getByTestId('analytics-weekly-trend')).toBeTruthy();
    expect(screen.getByText('Weekly Trend')).toBeTruthy();
  });

  it('shows type entries', () => {
    render(<AnalyticsView analytics={makeAnalytics()} />);
    expect(screen.getByText('job')).toBeTruthy();
    expect(screen.getByText('freelance')).toBeTruthy();
  });

  it('shows source entries', () => {
    render(<AnalyticsView analytics={makeAnalytics()} />);
    expect(screen.getByText('linkedin')).toBeTruthy();
    expect(screen.getByText('referral')).toBeTruthy();
  });
});
