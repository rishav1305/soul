// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ErrorBoundary } from './ErrorBoundary';

// Mock telemetry to avoid actual fetch calls
vi.mock('../lib/telemetry', () => ({
  reportError: vi.fn(),
}));

// Suppress console.error in tests (ErrorBoundary intentionally logs)
const originalConsoleError = console.error;

describe('ErrorBoundary', () => {
  beforeEach(() => {
    console.error = vi.fn();
  });

  afterEach(() => {
    console.error = originalConsoleError;
    cleanup();
  });

  it('renders children when no error', () => {
    render(
      <ErrorBoundary>
        <div data-testid="child">Hello</div>
      </ErrorBoundary>,
    );
    expect(screen.getByTestId('child')).toBeTruthy();
    expect(screen.getByText('Hello')).toBeTruthy();
  });

  it('renders error UI when child throws', () => {
    function ThrowingComponent(): never {
      throw new Error('Test error');
    }
    render(
      <ErrorBoundary>
        <ThrowingComponent />
      </ErrorBoundary>,
    );
    expect(screen.getByTestId('error-boundary')).toBeTruthy();
    expect(screen.getByText('Something went wrong')).toBeTruthy();
    expect(screen.getByText('Test error')).toBeTruthy();
  });

  it('shows retry button that recovers from error', () => {
    let shouldThrow = true;
    function MaybeThrow() {
      if (shouldThrow) throw new Error('Boom');
      return <div data-testid="recovered">Recovered</div>;
    }
    render(
      <ErrorBoundary>
        <MaybeThrow />
      </ErrorBoundary>,
    );
    // Error state
    expect(screen.getByTestId('error-boundary')).toBeTruthy();

    // Fix the error condition and retry
    shouldThrow = false;
    fireEvent.click(screen.getByTestId('error-retry'));

    // Should recover
    expect(screen.getByTestId('recovered')).toBeTruthy();
    expect(screen.queryByTestId('error-boundary')).toBeNull();
  });

  it('shows generic message when error has no message', () => {
    function ThrowingComponent(): never {
      throw new Error();
    }
    render(
      <ErrorBoundary>
        <ThrowingComponent />
      </ErrorBoundary>,
    );
    expect(screen.getByText('An unexpected error occurred')).toBeTruthy();
  });

  it('reports error to telemetry', async () => {
    const { reportError } = await import('../lib/telemetry');
    function ThrowingComponent(): never {
      throw new Error('Telemetry test');
    }
    render(
      <ErrorBoundary>
        <ThrowingComponent />
      </ErrorBoundary>,
    );
    expect(reportError).toHaveBeenCalledWith('ErrorBoundary', expect.any(Error));
  });

  it('retry button has data-testid', () => {
    function ThrowingComponent(): never {
      throw new Error('Test');
    }
    render(
      <ErrorBoundary>
        <ThrowingComponent />
      </ErrorBoundary>,
    );
    expect(screen.getByTestId('error-retry')).toBeTruthy();
    expect(screen.getByTestId('error-retry').textContent).toBe('Try Again');
  });

  it('catches errors from deeply nested children', () => {
    function DeepChild(): never { throw new Error('Deep error'); }
    function Parent() { return <DeepChild />; }
    function Grandparent() { return <Parent />; }

    render(
      <ErrorBoundary>
        <Grandparent />
      </ErrorBoundary>,
    );
    expect(screen.getByText('Deep error')).toBeTruthy();
    expect(screen.getByTestId('error-boundary')).toBeTruthy();
  });

  it('error-boundary container has bg-deep class', () => {
    function Throwing(): never { throw new Error('x'); }
    render(
      <ErrorBoundary>
        <Throwing />
      </ErrorBoundary>,
    );
    expect(screen.getByTestId('error-boundary').className).toContain('bg-deep');
  });

  it('retry button resets error state', () => {
    let shouldThrow = true;
    function MaybeThrow() {
      if (shouldThrow) throw new Error('Retry test');
      return <div data-testid="ok">OK</div>;
    }

    render(
      <ErrorBoundary>
        <MaybeThrow />
      </ErrorBoundary>,
    );
    expect(screen.getByTestId('error-boundary')).toBeTruthy();

    shouldThrow = false;
    fireEvent.click(screen.getByTestId('error-retry'));
    expect(screen.queryByTestId('error-boundary')).toBeNull();
    expect(screen.getByTestId('ok')).toBeTruthy();
  });

  it('error heading says "Something went wrong"', () => {
    function Throwing(): never { throw new Error('x'); }
    render(
      <ErrorBoundary>
        <Throwing />
      </ErrorBoundary>,
    );
    const heading = screen.getByText('Something went wrong');
    expect(heading.tagName).toBe('H1');
  });

  it('renders multiple children when no error', () => {
    render(
      <ErrorBoundary>
        <div data-testid="child-a">A</div>
        <div data-testid="child-b">B</div>
      </ErrorBoundary>,
    );
    expect(screen.getByTestId('child-a')).toBeTruthy();
    expect(screen.getByTestId('child-b')).toBeTruthy();
  });
});
