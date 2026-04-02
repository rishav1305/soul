// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ResultDetail } from './ResultDetail';

function makeResult(overrides: Record<string, unknown> = {}) {
  return {
    id: 'res-1',
    model_name: 'claude-3.5-sonnet',
    timestamp: new Date().toISOString(),
    accuracy: 87.5,
    latency_s: 1.23,
    tokens_per_sec: 42.7,
    peak_ram_mb: 256,
    cars_ram: 42,
    cars_size: 38,
    category_scores: [
      { category: 'reasoning', prompts_correct: 8, prompts_total: 10, accuracy: 80.0 },
      { category: 'coding', prompts_correct: 9, prompts_total: 10, accuracy: 90.0 },
    ],
    prompt_details: [
      { id: 'p1', prompt: 'What is 2+2?', category: 'math', correct: true, expected: '4', actual: '4', latency_ms: 150 },
      { id: 'p2', prompt: 'Write FizzBuzz', category: 'coding', correct: false, expected: 'for loop', actual: 'while loop', latency_ms: 320 },
    ],
    ...overrides,
  };
}

describe('ResultDetail', () => {
  afterEach(() => cleanup());

  it('renders result detail container', () => {
    render(<ResultDetail result={makeResult()} onClose={vi.fn()} />);
    expect(screen.getByTestId('result-detail-res-1')).toBeTruthy();
  });

  it('shows model name', () => {
    render(<ResultDetail result={makeResult({ model_name: 'gpt-4o' })} onClose={vi.fn()} />);
    expect(screen.getByText('gpt-4o')).toBeTruthy();
  });

  it('renders close button', () => {
    render(<ResultDetail result={makeResult()} onClose={vi.fn()} />);
    expect(screen.getByTestId('result-detail-close')).toBeTruthy();
  });

  it('calls onClose when close clicked', () => {
    const onClose = vi.fn();
    render(<ResultDetail result={makeResult()} onClose={onClose} />);
    fireEvent.click(screen.getByTestId('result-detail-close'));
    expect(onClose).toHaveBeenCalled();
  });

  it('shows accuracy percentage', () => {
    render(<ResultDetail result={makeResult({ accuracy: 92.3 })} onClose={vi.fn()} />);
    expect(screen.getByText('92.3%')).toBeTruthy();
  });

  it('shows latency', () => {
    render(<ResultDetail result={makeResult({ latency_s: 2.45 })} onClose={vi.fn()} />);
    expect(screen.getByText('2.45s')).toBeTruthy();
  });

  it('shows tokens per second', () => {
    render(<ResultDetail result={makeResult({ tokens_per_sec: 55.3 })} onClose={vi.fn()} />);
    expect(screen.getByText('55.3')).toBeTruthy();
  });

  it('shows peak RAM', () => {
    render(<ResultDetail result={makeResult({ peak_ram_mb: 512 })} onClose={vi.fn()} />);
    expect(screen.getByText('512 MB')).toBeTruthy();
  });

  it('shows category scores in table', () => {
    render(<ResultDetail result={makeResult()} onClose={vi.fn()} />);
    expect(screen.getByText('Per-Category Accuracy')).toBeTruthy();
    expect(screen.getByText('reasoning')).toBeTruthy();
    expect(screen.getByText('coding')).toBeTruthy();
  });

  it('shows prompt details', () => {
    render(<ResultDetail result={makeResult()} onClose={vi.fn()} />);
    expect(screen.getByText('Prompt Details')).toBeTruthy();
    expect(screen.getByTestId('prompt-toggle-p1')).toBeTruthy();
    expect(screen.getByTestId('prompt-toggle-p2')).toBeTruthy();
  });

  it('expands prompt detail on click', () => {
    render(<ResultDetail result={makeResult()} onClose={vi.fn()} />);
    fireEvent.click(screen.getByTestId('prompt-toggle-p1'));
    expect(screen.getByText('Expected:')).toBeTruthy();
    expect(screen.getByText('Actual:')).toBeTruthy();
  });

  it('collapses prompt detail on second click', () => {
    render(<ResultDetail result={makeResult()} onClose={vi.fn()} />);
    fireEvent.click(screen.getByTestId('prompt-toggle-p1'));
    expect(screen.getByText('Expected:')).toBeTruthy();
    fireEvent.click(screen.getByTestId('prompt-toggle-p1'));
    expect(screen.queryByText('Expected:')).toBeNull();
  });

  it('shows prompt latency', () => {
    render(<ResultDetail result={makeResult()} onClose={vi.fn()} />);
    expect(screen.getByText('150ms')).toBeTruthy();
    expect(screen.getByText('320ms')).toBeTruthy();
  });
});
