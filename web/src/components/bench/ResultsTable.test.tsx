// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ResultsTable } from './ResultsTable';

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

describe('ResultsTable', () => {
  afterEach(() => cleanup());

  it('shows empty state when no results', () => {
    render(<ResultsTable results={[]} onSelect={vi.fn()} />);
    expect(screen.getByText('No benchmark results yet.')).toBeTruthy();
  });

  it('renders results table', () => {
    render(<ResultsTable results={[makeResult()]} onSelect={vi.fn()} />);
    expect(screen.getByTestId('results-table')).toBeTruthy();
  });

  it('renders result row with testid', () => {
    render(<ResultsTable results={[makeResult({ id: 'res-42' })]} onSelect={vi.fn()} />);
    expect(screen.getByTestId('result-row-res-42')).toBeTruthy();
  });

  it('shows model name', () => {
    render(<ResultsTable results={[makeResult({ model_name: 'gpt-4o' })]} onSelect={vi.fn()} />);
    expect(screen.getByText('gpt-4o')).toBeTruthy();
  });

  it('shows accuracy with percentage', () => {
    render(<ResultsTable results={[makeResult({ accuracy: 92.3 })]} onSelect={vi.fn()} />);
    expect(screen.getByText('92.3%')).toBeTruthy();
  });

  it('shows latency in seconds', () => {
    render(<ResultsTable results={[makeResult({ latency_s: 2.45 })]} onSelect={vi.fn()} />);
    expect(screen.getByText('2.45s')).toBeTruthy();
  });

  it('shows CARS RAM score', () => {
    render(<ResultsTable results={[makeResult({ cars_ram: 55 })]} onSelect={vi.fn()} />);
    expect(screen.getByText('55')).toBeTruthy();
  });

  it('shows CARS Size score', () => {
    render(<ResultsTable results={[makeResult({ cars_size: 40 })]} onSelect={vi.fn()} />);
    expect(screen.getByText('40')).toBeTruthy();
  });

  it('calls onSelect when result row clicked', () => {
    const onSelect = vi.fn();
    render(<ResultsTable results={[makeResult({ id: 'res-5' })]} onSelect={onSelect} />);
    fireEvent.click(screen.getByTestId('result-row-res-5'));
    expect(onSelect).toHaveBeenCalledWith('res-5');
  });

  it('renders column headers', () => {
    render(<ResultsTable results={[makeResult()]} onSelect={vi.fn()} />);
    expect(screen.getByText('Model')).toBeTruthy();
    expect(screen.getByText('Accuracy')).toBeTruthy();
    expect(screen.getByText('Latency')).toBeTruthy();
  });

  it('renders multiple results', () => {
    const results = [
      makeResult({ id: 'res-1', model_name: 'Model A' }),
      makeResult({ id: 'res-2', model_name: 'Model B' }),
    ];
    render(<ResultsTable results={results} onSelect={vi.fn()} />);
    expect(screen.getByTestId('result-row-res-1')).toBeTruthy();
    expect(screen.getByTestId('result-row-res-2')).toBeTruthy();
  });
});
