// @vitest-environment jsdom
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
    Object.assign(navigator, { clipboard: { writeText } });

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
});
