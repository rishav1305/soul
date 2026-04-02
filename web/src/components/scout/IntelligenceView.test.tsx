// @vitest-environment happy-dom
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { IntelligenceView } from './IntelligenceView';

vi.mock('./HotLeadsTable', () => ({
  HotLeadsTable: ({ leads }: { leads: unknown[] }) => (
    <div data-testid="mock-hot-leads-table">{leads.length} leads</div>
  ),
}));

vi.mock('./AgentCards', () => ({
  AgentCards: ({ runs }: { runs: unknown[] }) => (
    <div data-testid="mock-agent-cards">{runs.length} runs</div>
  ),
}));

vi.mock('./DigestSummary', () => ({
  DigestSummary: () => <div data-testid="mock-digest-summary">Digest</div>,
}));

import { vi } from 'vitest';

describe('IntelligenceView', () => {
  afterEach(() => cleanup());

  it('renders intelligence view container', () => {
    render(<IntelligenceView scoredLeads={[]} agentRuns={[]} leads={[]} analytics={null} />);
    expect(screen.getByTestId('intelligence-view')).toBeTruthy();
  });

  it('renders DigestSummary', () => {
    render(<IntelligenceView scoredLeads={[]} agentRuns={[]} leads={[]} analytics={null} />);
    expect(screen.getByTestId('mock-digest-summary')).toBeTruthy();
  });

  it('renders HotLeadsTable', () => {
    render(<IntelligenceView scoredLeads={[]} agentRuns={[]} leads={[]} analytics={null} />);
    expect(screen.getByTestId('mock-hot-leads-table')).toBeTruthy();
  });

  it('renders AgentCards', () => {
    render(<IntelligenceView scoredLeads={[]} agentRuns={[]} leads={[]} analytics={null} />);
    expect(screen.getByTestId('mock-agent-cards')).toBeTruthy();
  });

  it('passes scoredLeads to HotLeadsTable', () => {
    const scored = [{ id: 1 }, { id: 2 }] as any[];
    render(<IntelligenceView scoredLeads={scored} agentRuns={[]} leads={[]} analytics={null} />);
    expect(screen.getByTestId('mock-hot-leads-table').textContent).toContain('2 leads');
  });

  it('passes agentRuns to AgentCards', () => {
    const runs = [{ id: 1 }, { id: 2 }, { id: 3 }] as any[];
    render(<IntelligenceView scoredLeads={[]} agentRuns={runs} leads={[]} analytics={null} />);
    expect(screen.getByTestId('mock-agent-cards').textContent).toContain('3 runs');
  });
});
