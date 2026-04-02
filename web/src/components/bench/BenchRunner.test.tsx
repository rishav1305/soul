// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import { BenchRunner } from './BenchRunner';

const defaultCategories = () => [
  { id: 'reasoning', name: 'Reasoning', prompt_count: 5 },
  { id: 'coding', name: 'Coding', prompt_count: 8 },
  { id: 'math', name: 'Math', prompt_count: 3 },
];

describe('BenchRunner', () => {
  afterEach(() => cleanup());

  it('renders bench runner container', () => {
    render(<BenchRunner categories={[]} loading={false} onRun={vi.fn()} />);
    expect(screen.getByTestId('bench-runner')).toBeTruthy();
  });

  it('renders endpoint input', () => {
    render(<BenchRunner categories={[]} loading={false} onRun={vi.fn()} />);
    expect(screen.getByTestId('model-endpoint')).toBeTruthy();
  });

  it('shows no categories message when empty', () => {
    render(<BenchRunner categories={[]} loading={false} onRun={vi.fn()} />);
    expect(screen.getByText('No categories available.')).toBeTruthy();
  });

  it('renders category labels', () => {
    render(<BenchRunner categories={defaultCategories()} loading={false} onRun={vi.fn()} />);
    expect(screen.getByText('Reasoning (5)')).toBeTruthy();
    expect(screen.getByText('Coding (8)')).toBeTruthy();
    expect(screen.getByText('Math (3)')).toBeTruthy();
  });

  it('renders GPU toggle', () => {
    render(<BenchRunner categories={[]} loading={false} onRun={vi.fn()} />);
    expect(screen.getByTestId('gpu-toggle')).toBeTruthy();
  });

  it('renders max tokens input', () => {
    render(<BenchRunner categories={[]} loading={false} onRun={vi.fn()} />);
    const input = screen.getByTestId('max-tokens') as HTMLInputElement;
    expect(input).toBeTruthy();
    expect(input.value).toBe('1024');
  });

  it('renders start benchmark button', () => {
    render(<BenchRunner categories={[]} loading={false} onRun={vi.fn()} />);
    expect(screen.getByTestId('start-benchmark')).toBeTruthy();
    expect(screen.getByText('Start Benchmark')).toBeTruthy();
  });

  it('start button is disabled when endpoint is empty', () => {
    render(<BenchRunner categories={[]} loading={false} onRun={vi.fn()} />);
    expect((screen.getByTestId('start-benchmark') as HTMLButtonElement).disabled).toBe(true);
  });

  it('start button is enabled when endpoint is entered', () => {
    render(<BenchRunner categories={[]} loading={false} onRun={vi.fn()} />);
    fireEvent.change(screen.getByTestId('model-endpoint'), { target: { value: 'https://api.test.com' } });
    expect((screen.getByTestId('start-benchmark') as HTMLButtonElement).disabled).toBe(false);
  });

  it('shows Running... when loading', () => {
    render(<BenchRunner categories={[]} loading={true} onRun={vi.fn()} />);
    expect(screen.getByText('Running...')).toBeTruthy();
  });

  it('shows progress bar when loading', () => {
    render(<BenchRunner categories={[]} loading={true} onRun={vi.fn()} />);
    expect(screen.getByTestId('bench-progress')).toBeTruthy();
  });

  it('calls onRun with config when Start clicked', async () => {
    const onRun = vi.fn().mockResolvedValue(undefined);
    render(<BenchRunner categories={defaultCategories()} loading={false} onRun={onRun} />);
    fireEvent.change(screen.getByTestId('model-endpoint'), { target: { value: 'https://api.test.com' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('start-benchmark'));
    });
    expect(onRun).toHaveBeenCalledWith(expect.objectContaining({
      endpoint: 'https://api.test.com',
      max_tokens: 1024,
    }));
  });

  it('toggles category selection', () => {
    render(<BenchRunner categories={defaultCategories()} loading={false} onRun={vi.fn()} />);
    // Click Reasoning category
    fireEvent.click(screen.getByTestId('category-reasoning'));
    // Visual state should change (bg-soul/20 applied when selected)
    const label = screen.getByTestId('category-reasoning');
    expect(label.className).toContain('soul');
  });
});
