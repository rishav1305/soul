// @vitest-environment happy-dom
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { AgentActivity } from './AgentActivity';

function makeSweep(overrides: Record<string, unknown> = {}) {
  return {
    running: true,
    platforms: ['LinkedIn', 'GitHub'],
    started_at: new Date().toISOString(),
    progress: 45,
    results_found: 12,
    ...overrides,
  };
}

describe('AgentActivity', () => {
  afterEach(() => cleanup());

  it('renders agent activity container', () => {
    render(<AgentActivity sweepStatus={null} />);
    expect(screen.getByTestId('agent-activity')).toBeTruthy();
  });

  it('shows idle state when sweepStatus is null', () => {
    render(<AgentActivity sweepStatus={null} />);
    expect(screen.getByText('No active agent runs')).toBeTruthy();
  });

  it('shows idle state when not running', () => {
    render(<AgentActivity sweepStatus={makeSweep({ running: false })} />);
    expect(screen.getByText('No active agent runs')).toBeTruthy();
  });

  it('shows title', () => {
    render(<AgentActivity sweepStatus={null} />);
    expect(screen.getByText('Agent Activity')).toBeTruthy();
  });

  it('shows Running badge when active', () => {
    render(<AgentActivity sweepStatus={makeSweep()} />);
    expect(screen.getByText('Running')).toBeTruthy();
  });

  it('shows platforms', () => {
    render(<AgentActivity sweepStatus={makeSweep({ platforms: ['LinkedIn', 'GitHub'] })} />);
    expect(screen.getByText('LinkedIn, GitHub')).toBeTruthy();
  });

  it('shows results found count', () => {
    render(<AgentActivity sweepStatus={makeSweep({ results_found: 25 })} />);
    expect(screen.getByText('25')).toBeTruthy();
  });

  it('shows progress percentage', () => {
    render(<AgentActivity sweepStatus={makeSweep({ progress: 72 })} />);
    expect(screen.getByText('72%')).toBeTruthy();
  });

  it('renders progress bar', () => {
    render(<AgentActivity sweepStatus={makeSweep({ progress: 60 })} />);
    const bar = screen.getByTestId('agent-progress-bar');
    expect(bar.style.width).toBe('60%');
  });

  it('shows started time', () => {
    render(<AgentActivity sweepStatus={makeSweep()} />);
    expect(screen.getByText('Started')).toBeTruthy();
  });
});
