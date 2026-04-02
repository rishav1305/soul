// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import { SmokeTest } from './SmokeTest';

describe('SmokeTest', () => {
  afterEach(() => cleanup());

  it('renders smoke test container', () => {
    render(<SmokeTest onRunSmoke={vi.fn()} />);
    expect(screen.getByTestId('smoke-test')).toBeTruthy();
  });

  it('renders endpoint input', () => {
    render(<SmokeTest onRunSmoke={vi.fn()} />);
    expect(screen.getByTestId('smoke-endpoint')).toBeTruthy();
  });

  it('renders run button', () => {
    render(<SmokeTest onRunSmoke={vi.fn()} />);
    expect(screen.getByTestId('run-smoke')).toBeTruthy();
    expect(screen.getByText('Run Smoke')).toBeTruthy();
  });

  it('run button is disabled when endpoint is empty', () => {
    render(<SmokeTest onRunSmoke={vi.fn()} />);
    expect((screen.getByTestId('run-smoke') as HTMLButtonElement).disabled).toBe(true);
  });

  it('run button is enabled when endpoint is entered', () => {
    render(<SmokeTest onRunSmoke={vi.fn()} />);
    fireEvent.change(screen.getByTestId('smoke-endpoint'), { target: { value: 'https://api.test.com' } });
    expect((screen.getByTestId('run-smoke') as HTMLButtonElement).disabled).toBe(false);
  });

  it('calls onRunSmoke with endpoint when run clicked', async () => {
    const onRunSmoke = vi.fn().mockResolvedValue([]);
    render(<SmokeTest onRunSmoke={onRunSmoke} />);
    fireEvent.change(screen.getByTestId('smoke-endpoint'), { target: { value: 'https://api.test.com' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('run-smoke'));
    });
    expect(onRunSmoke).toHaveBeenCalledWith('https://api.test.com');
  });

  it('shows results after successful run', async () => {
    const results = [
      { name: 'Basic Chat', passed: true, response_preview: 'Hello!' },
      { name: 'Safety Check', passed: false, response_preview: 'Error' },
    ];
    const onRunSmoke = vi.fn().mockResolvedValue(results);
    render(<SmokeTest onRunSmoke={onRunSmoke} />);
    fireEvent.change(screen.getByTestId('smoke-endpoint'), { target: { value: 'https://api.test.com' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('run-smoke'));
    });
    expect(screen.getByTestId('smoke-results')).toBeTruthy();
    expect(screen.getByTestId('smoke-card-0')).toBeTruthy();
    expect(screen.getByTestId('smoke-card-1')).toBeTruthy();
  });

  it('shows PASS/FAIL badges', async () => {
    const results = [
      { name: 'Test A', passed: true, response_preview: 'OK' },
      { name: 'Test B', passed: false, response_preview: 'ERR' },
    ];
    const onRunSmoke = vi.fn().mockResolvedValue(results);
    render(<SmokeTest onRunSmoke={onRunSmoke} />);
    fireEvent.change(screen.getByTestId('smoke-endpoint'), { target: { value: 'https://test.com' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('run-smoke'));
    });
    expect(screen.getByText('PASS')).toBeTruthy();
    expect(screen.getByText('FAIL')).toBeTruthy();
  });

  it('hides results when none exist', () => {
    render(<SmokeTest onRunSmoke={vi.fn()} />);
    expect(screen.queryByTestId('smoke-results')).toBeNull();
  });
});
