// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import { ScanResults } from './ScanResults';

function makeResult(overrides: Record<string, unknown> = {}) {
  return {
    severity: 'medium',
    title: 'Missing CSP Header',
    description: 'Content-Security-Policy header not set',
    recommendation: 'Add CSP header to all responses',
    ...overrides,
  };
}

describe('ScanResults', () => {
  afterEach(() => cleanup());

  it('renders scan results container', () => {
    render(<ScanResults results={[]} onScan={vi.fn()} />);
    expect(screen.getByTestId('scan-results')).toBeTruthy();
  });

  it('renders product selector', () => {
    render(<ScanResults results={[]} onScan={vi.fn()} />);
    expect(screen.getByTestId('scan-product-select')).toBeTruthy();
  });

  it('renders scan button', () => {
    render(<ScanResults results={[]} onScan={vi.fn()} />);
    expect(screen.getByTestId('scan-button')).toBeTruthy();
    expect(screen.getByText('Run Scan')).toBeTruthy();
  });

  it('shows empty state when no results', () => {
    render(<ScanResults results={[]} onScan={vi.fn()} />);
    expect(screen.getByText('No scan results yet. Select a product and run a scan.')).toBeTruthy();
  });

  it('calls onScan with selected product when button clicked', async () => {
    const onScan = vi.fn().mockResolvedValue(undefined);
    render(<ScanResults results={[]} onScan={onScan} />);
    fireEvent.change(screen.getByTestId('scan-product-select'), { target: { value: 'tasks' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('scan-button'));
    });
    expect(onScan).toHaveBeenCalledWith('tasks');
  });

  it('renders results table when results exist', () => {
    render(<ScanResults results={[makeResult()]} onScan={vi.fn()} />);
    expect(screen.getByTestId('scan-table')).toBeTruthy();
  });

  it('shows result rows', () => {
    const results = [makeResult(), makeResult({ title: 'SQL Injection Risk' })];
    render(<ScanResults results={results} onScan={vi.fn()} />);
    expect(screen.getByTestId('scan-row-0')).toBeTruthy();
    expect(screen.getByTestId('scan-row-1')).toBeTruthy();
  });

  it('shows severity badge', () => {
    render(<ScanResults results={[makeResult({ severity: 'critical' })]} onScan={vi.fn()} />);
    expect(screen.getByText('critical')).toBeTruthy();
  });

  it('shows result title', () => {
    render(<ScanResults results={[makeResult({ title: 'XSS Vulnerability' })]} onScan={vi.fn()} />);
    expect(screen.getByText('XSS Vulnerability')).toBeTruthy();
  });

  it('shows result description', () => {
    render(<ScanResults results={[makeResult({ description: 'Input not sanitized' })]} onScan={vi.fn()} />);
    expect(screen.getByText('Input not sanitized')).toBeTruthy();
  });

  it('shows recommendation', () => {
    render(<ScanResults results={[makeResult({ recommendation: 'Sanitize all inputs' })]} onScan={vi.fn()} />);
    expect(screen.getByText('Sanitize all inputs')).toBeTruthy();
  });

  it('shows findings count', () => {
    const results = [makeResult(), makeResult({ title: 'B' }), makeResult({ title: 'C' })];
    render(<ScanResults results={results} onScan={vi.fn()} />);
    expect(screen.getByText('3 findings')).toBeTruthy();
  });

  it('sorts by severity (critical first)', () => {
    const results = [
      makeResult({ severity: 'low', title: 'Low issue' }),
      makeResult({ severity: 'critical', title: 'Critical issue' }),
    ];
    const { container } = render(<ScanResults results={results} onScan={vi.fn()} />);
    const rows = container.querySelectorAll('[data-testid^="scan-row-"]');
    // Critical should be first
    expect(rows[0].textContent).toContain('critical');
  });
});
