// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { AgentCards } from './AgentCards';

function makeRun(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    platform: 'linkedin',
    mode: 'sweep',
    status: 'completed',
    results_found: 12,
    summary: 'Found 12 leads matching criteria',
    created_at: new Date().toISOString(),
    ...overrides,
  };
}

describe('AgentCards', () => {
  afterEach(() => cleanup());

  it('renders agent cards container', () => {
    render(<AgentCards runs={[]} />);
    expect(screen.getByTestId('agent-cards')).toBeTruthy();
  });

  it('shows Agent History title', () => {
    render(<AgentCards runs={[]} />);
    expect(screen.getByText('Agent History')).toBeTruthy();
  });

  it('shows empty state when no runs', () => {
    render(<AgentCards runs={[]} />);
    expect(screen.getByText('No agent runs recorded')).toBeTruthy();
  });

  it('renders run cards', () => {
    const runs = [makeRun({ id: 1 }), makeRun({ id: 2, platform: 'github' })];
    render(<AgentCards runs={runs} />);
    expect(screen.getByTestId('agent-run-1')).toBeTruthy();
    expect(screen.getByTestId('agent-run-2')).toBeTruthy();
  });

  it('shows platform name', () => {
    render(<AgentCards runs={[makeRun({ platform: 'github' })]} />);
    expect(screen.getByText('github')).toBeTruthy();
  });

  it('shows run status', () => {
    render(<AgentCards runs={[makeRun({ status: 'running' })]} />);
    expect(screen.getByText('running')).toBeTruthy();
  });

  it('shows mode', () => {
    render(<AgentCards runs={[makeRun({ mode: 'engage' })]} />);
    expect(screen.getByText('engage')).toBeTruthy();
  });

  it('shows results found count', () => {
    render(<AgentCards runs={[makeRun({ results_found: 25 })]} />);
    expect(screen.getByText('25 found')).toBeTruthy();
  });

  it('shows summary when present', () => {
    render(<AgentCards runs={[makeRun({ summary: 'Completed sweep' })]} />);
    expect(screen.getByText('Completed sweep')).toBeTruthy();
  });
});
