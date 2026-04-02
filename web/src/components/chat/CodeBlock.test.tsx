// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup, fireEvent, act } from '@testing-library/react';

// Mock SyntaxHighlighter to avoid loading the full Prism bundle in tests
vi.mock('react-syntax-highlighter/dist/esm/prism-light', () => ({
  default: Object.assign(
    ({ children, language }: { children: string; language: string }) => (
      <pre data-testid="syntax-highlighter" data-language={language}>{children}</pre>
    ),
    { registerLanguage: vi.fn() },
  ),
}));
vi.mock('react-syntax-highlighter/dist/esm/styles/prism/one-dark', () => ({ default: {} }));
// Mock all language imports
vi.mock('react-syntax-highlighter/dist/esm/languages/prism/tsx', () => ({ default: {} }));
vi.mock('react-syntax-highlighter/dist/esm/languages/prism/typescript', () => ({ default: {} }));
vi.mock('react-syntax-highlighter/dist/esm/languages/prism/javascript', () => ({ default: {} }));
vi.mock('react-syntax-highlighter/dist/esm/languages/prism/go', () => ({ default: {} }));
vi.mock('react-syntax-highlighter/dist/esm/languages/prism/python', () => ({ default: {} }));
vi.mock('react-syntax-highlighter/dist/esm/languages/prism/bash', () => ({ default: {} }));
vi.mock('react-syntax-highlighter/dist/esm/languages/prism/json', () => ({ default: {} }));
vi.mock('react-syntax-highlighter/dist/esm/languages/prism/yaml', () => ({ default: {} }));
vi.mock('react-syntax-highlighter/dist/esm/languages/prism/css', () => ({ default: {} }));
vi.mock('react-syntax-highlighter/dist/esm/languages/prism/sql', () => ({ default: {} }));
vi.mock('react-syntax-highlighter/dist/esm/languages/prism/markdown', () => ({ default: {} }));
vi.mock('react-syntax-highlighter/dist/esm/languages/prism/rust', () => ({ default: {} }));
vi.mock('react-syntax-highlighter/dist/esm/languages/prism/java', () => ({ default: {} }));
vi.mock('react-syntax-highlighter/dist/esm/languages/prism/docker', () => ({ default: {} }));

import CodeBlock from './CodeBlock';

describe('CodeBlock', () => {
  afterEach(() => cleanup());

  it('renders code-block container', () => {
    render(<CodeBlock language="go" code='fmt.Println("hi")' />);
    expect(screen.getByTestId('code-block')).toBeTruthy();
  });

  it('displays language label', () => {
    render(<CodeBlock language="typescript" code="const x = 1;" />);
    const label = screen.getByTestId('code-language-label');
    expect(label.textContent).toBe('TypeScript');
  });

  it('maps alias to friendly name', () => {
    render(<CodeBlock language="py" code="print('hi')" />);
    const label = screen.getByTestId('code-language-label');
    expect(label.textContent).toBe('Python');
  });

  it('capitalizes unknown languages', () => {
    render(<CodeBlock language="lua" code="local x = 1" />);
    const label = screen.getByTestId('code-language-label');
    expect(label.textContent).toBe('Lua');
  });

  it('shows Code for empty language', () => {
    render(<CodeBlock language="" code="raw text" />);
    const label = screen.getByTestId('code-language-label');
    expect(label.textContent).toBe('Code');
  });

  it('renders copy button', () => {
    render(<CodeBlock language="go" code="test" />);
    const btn = screen.getByTestId('code-copy-button');
    expect(btn).toBeTruthy();
    expect(btn.textContent).toContain('Copy');
  });

  it('copy button shows Copied! on click', async () => {
    // Mock clipboard API
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, writable: true, configurable: true });

    render(<CodeBlock language="go" code="test code" />);
    const btn = screen.getByTestId('code-copy-button');

    await act(async () => {
      fireEvent.click(btn);
    });

    expect(writeText).toHaveBeenCalledWith('test code');
    expect(btn.textContent).toContain('Copied!');
  });

  it('passes code content to SyntaxHighlighter', () => {
    render(<CodeBlock language="json" code='{"key": "value"}' />);
    const highlighter = screen.getByTestId('syntax-highlighter');
    expect(highlighter.textContent).toBe('{"key": "value"}');
  });

  it('passes language to SyntaxHighlighter', () => {
    render(<CodeBlock language="Go" code="package main" />);
    const highlighter = screen.getByTestId('syntax-highlighter');
    expect(highlighter.getAttribute('data-language')).toBe('go');
  });

  it('maps sh alias to Shell label', () => {
    render(<CodeBlock language="sh" code="echo hi" />);
    expect(screen.getByTestId('code-language-label').textContent).toBe('Shell');
  });

  it('maps yml alias to YAML label', () => {
    render(<CodeBlock language="yml" code="key: value" />);
    expect(screen.getByTestId('code-language-label').textContent).toBe('YAML');
  });

  it('maps rs alias to Rust label', () => {
    render(<CodeBlock language="rs" code="fn main() {}" />);
    expect(screen.getByTestId('code-language-label').textContent).toBe('Rust');
  });

  it('maps golang alias to Go label', () => {
    render(<CodeBlock language="golang" code="package main" />);
    expect(screen.getByTestId('code-language-label').textContent).toBe('Go');
  });

  it('maps jsx alias to JSX label', () => {
    render(<CodeBlock language="jsx" code="<div />" />);
    expect(screen.getByTestId('code-language-label').textContent).toBe('JSX');
  });

  it('maps dockerfile alias to Dockerfile label', () => {
    render(<CodeBlock language="dockerfile" code="FROM node:18" />);
    expect(screen.getByTestId('code-language-label').textContent).toBe('Dockerfile');
  });

  it('maps ts alias to TypeScript label', () => {
    render(<CodeBlock language="ts" code="const x: number = 1" />);
    expect(screen.getByTestId('code-language-label').textContent).toBe('TypeScript');
  });

  it('maps js alias to JavaScript label', () => {
    render(<CodeBlock language="js" code="const x = 1" />);
    expect(screen.getByTestId('code-language-label').textContent).toBe('JavaScript');
  });

  it('maps md alias to Markdown label', () => {
    render(<CodeBlock language="md" code="# Hello" />);
    expect(screen.getByTestId('code-language-label').textContent).toBe('Markdown');
  });

  it('copy button reverts from Copied! after timeout', async () => {
    vi.useFakeTimers();
    const writeText = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', { value: { writeText }, writable: true, configurable: true });

    render(<CodeBlock language="go" code="test" />);
    const btn = screen.getByTestId('code-copy-button');

    await act(async () => {
      fireEvent.click(btn);
    });
    expect(btn.textContent).toContain('Copied!');

    await act(async () => {
      vi.advanceTimersByTime(2500);
    });
    expect(btn.textContent).toContain('Copy');
    expect(btn.textContent).not.toContain('Copied!');
    vi.useRealTimers();
  });

  it('renders multi-line code correctly', () => {
    const code = 'line 1\nline 2\nline 3';
    render(<CodeBlock language="text" code={code} />);
    const highlighter = screen.getByTestId('syntax-highlighter');
    expect(highlighter.textContent).toBe(code);
  });
});
