// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { MemoryHealth } from './MemoryHealth';
import type { MemoryHealthData } from './MemoryHealth';

function makeHealth(overrides: Partial<MemoryHealthData> = {}): MemoryHealthData {
  return {
    chromadb: 'up',
    collections: {
      soul_agent_memory: 109,
      soul_shared_kb: 28,
      soul_briefs: 5,
    },
    last_index: new Date().toISOString(),
    ...overrides,
  };
}

describe('MemoryHealth', () => {
  afterEach(() => cleanup());

  it('renders container', () => {
    render(<MemoryHealth health={makeHealth()} />);
    expect(screen.getByTestId('memory-health')).toBeTruthy();
  });

  it('shows Online status when chromadb is up', () => {
    render(<MemoryHealth health={makeHealth({ chromadb: 'up' })} />);
    expect(screen.getByTestId('memory-health-status').textContent).toBe('Online');
  });

  it('shows Offline status when chromadb is down', () => {
    render(<MemoryHealth health={makeHealth({ chromadb: 'down' })} />);
    expect(screen.getByTestId('memory-health-status').textContent).toBe('Offline');
  });

  it('shows total document count', () => {
    render(<MemoryHealth health={makeHealth()} />);
    // 109 + 28 + 5 = 142
    expect(screen.getByTestId('memory-health-total').textContent).toContain('142');
  });

  it('shows individual collection counts', () => {
    render(<MemoryHealth health={makeHealth()} />);
    expect(screen.getByTestId('memory-health-collection-soul_agent_memory').textContent).toContain('109');
    expect(screen.getByTestId('memory-health-collection-soul_shared_kb').textContent).toContain('28');
    expect(screen.getByTestId('memory-health-collection-soul_briefs').textContent).toContain('5');
  });

  it('shows last index timestamp', () => {
    render(<MemoryHealth health={makeHealth()} />);
    expect(screen.getByTestId('memory-health-last-index')).toBeTruthy();
  });

  it('renders refresh button when onRefresh provided', () => {
    render(<MemoryHealth health={makeHealth()} onRefresh={vi.fn()} />);
    expect(screen.getByTestId('memory-health-refresh')).toBeTruthy();
  });

  it('hides refresh button when onRefresh not provided', () => {
    render(<MemoryHealth health={makeHealth()} />);
    expect(screen.queryByTestId('memory-health-refresh')).toBeNull();
  });

  it('calls onRefresh when refresh clicked', () => {
    const onRefresh = vi.fn();
    render(<MemoryHealth health={makeHealth()} onRefresh={onRefresh} />);
    fireEvent.click(screen.getByTestId('memory-health-refresh'));
    expect(onRefresh).toHaveBeenCalled();
  });

  it('disables refresh when loading', () => {
    render(<MemoryHealth health={makeHealth()} loading={true} onRefresh={vi.fn()} />);
    expect((screen.getByTestId('memory-health-refresh') as HTMLButtonElement).disabled).toBe(true);
  });

  it('shows error message', () => {
    render(<MemoryHealth health={null} error="Connection refused" />);
    expect(screen.getByTestId('memory-health-error').textContent).toBe('Connection refused');
  });

  it('hides error when null', () => {
    render(<MemoryHealth health={makeHealth()} error={null} />);
    expect(screen.queryByTestId('memory-health-error')).toBeNull();
  });

  it('shows loading state when no health data', () => {
    render(<MemoryHealth health={null} loading={true} />);
    expect(screen.getByTestId('memory-health-loading')).toBeTruthy();
  });

  it('shows empty state when no health and not loading', () => {
    render(<MemoryHealth health={null} />);
    expect(screen.getByTestId('memory-health-empty')).toBeTruthy();
  });

  it('hides empty state when health data present', () => {
    render(<MemoryHealth health={makeHealth()} />);
    expect(screen.queryByTestId('memory-health-empty')).toBeNull();
  });

  it('collection names strip soul_ prefix for display', () => {
    render(<MemoryHealth health={makeHealth()} />);
    const el = screen.getByTestId('memory-health-collection-soul_agent_memory');
    expect(el.textContent).toContain('agent_memory');
    expect(el.textContent).not.toContain('soul_agent_memory');
  });
});
