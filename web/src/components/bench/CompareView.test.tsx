// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { CompareView } from './CompareView';

function makeResult(overrides: Record<string, unknown> = {}) {
  return {
    id: 'res-1',
    model_name: 'claude-3.5-sonnet',
    timestamp: new Date().toISOString(),
    accuracy: 87.5,
    latency_s: 1.23,
    cars_ram: 42,
    cars_size: 38,
    ...overrides,
  };
}

describe('CompareView', () => {
  afterEach(() => cleanup());

  it('renders compare view container', () => {
    render(<CompareView results={[]} compareData={null} loading={false} onCompare={vi.fn()} />);
    expect(screen.getByTestId('compare-view')).toBeTruthy();
  });

  it('renders two select dropdowns', () => {
    render(<CompareView results={[]} compareData={null} loading={false} onCompare={vi.fn()} />);
    expect(screen.getByTestId('compare-select-1')).toBeTruthy();
    expect(screen.getByTestId('compare-select-2')).toBeTruthy();
  });

  it('renders compare button', () => {
    render(<CompareView results={[]} compareData={null} loading={false} onCompare={vi.fn()} />);
    expect(screen.getByTestId('compare-btn')).toBeTruthy();
    expect(screen.getByText('Compare')).toBeTruthy();
  });

  it('compare button is disabled when no selections', () => {
    render(<CompareView results={[]} compareData={null} loading={false} onCompare={vi.fn()} />);
    expect((screen.getByTestId('compare-btn') as HTMLButtonElement).disabled).toBe(true);
  });

  it('shows vs label between selects', () => {
    render(<CompareView results={[]} compareData={null} loading={false} onCompare={vi.fn()} />);
    expect(screen.getByText('vs')).toBeTruthy();
  });

  it('populates select options from results', () => {
    const results = [
      makeResult({ id: 'res-1', model_name: 'Claude' }),
      makeResult({ id: 'res-2', model_name: 'GPT-4' }),
    ];
    render(<CompareView results={results} compareData={null} loading={false} onCompare={vi.fn()} />);
    // Each select should have the results + placeholder
    const select1 = screen.getByTestId('compare-select-1');
    expect(select1.querySelectorAll('option').length).toBe(3); // placeholder + 2 results
  });

  it('shows Comparing... when loading', () => {
    render(<CompareView results={[]} compareData={null} loading={true} onCompare={vi.fn()} />);
    expect(screen.getByText('Comparing...')).toBeTruthy();
  });

  it('shows Metrics Comparison when compareData is set', () => {
    const compareData = {
      result1: {
        model_name: 'Claude', accuracy: 85, latency_s: 1.5,
        tokens_per_sec: 50, peak_ram_mb: 200, cars_ram: 40, cars_size: 35,
        category_scores: [],
      },
      result2: {
        model_name: 'GPT-4', accuracy: 80, latency_s: 2.0,
        tokens_per_sec: 40, peak_ram_mb: 300, cars_ram: 35, cars_size: 30,
        category_scores: [],
      },
    };
    render(<CompareView results={[]} compareData={compareData} loading={false} onCompare={vi.fn()} />);
    expect(screen.getByText('Metrics Comparison')).toBeTruthy();
    expect(screen.getByText('Per-Category Comparison')).toBeTruthy();
  });

  it('shows model names in comparison headers', () => {
    const compareData = {
      result1: {
        model_name: 'Claude', accuracy: 85, latency_s: 1.5,
        tokens_per_sec: 50, peak_ram_mb: 200, cars_ram: 40, cars_size: 35,
        category_scores: [],
      },
      result2: {
        model_name: 'GPT-4', accuracy: 80, latency_s: 2.0,
        tokens_per_sec: 40, peak_ram_mb: 300, cars_ram: 35, cars_size: 30,
        category_scores: [],
      },
    };
    render(<CompareView results={[]} compareData={compareData} loading={false} onCompare={vi.fn()} />);
    // Model names should appear in table headers (multiple times for each table)
    const allClaude = screen.getAllByText('Claude');
    expect(allClaude.length).toBeGreaterThanOrEqual(1);
  });
});
