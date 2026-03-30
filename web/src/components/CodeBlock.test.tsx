// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import CodeBlock from './CodeBlock';

// Mock SyntaxHighlighter to avoid heavy dependency
vi.mock('react-syntax-highlighter/dist/esm/prism-light', () => ({
  default: Object.assign(
    ({ children, language, showLineNumbers }: { children: string; language: string; showLineNumbers: boolean }) => (
      <pre data-testid="mock-syntax-highlighter" data-language={language} data-line-numbers={showLineNumbers}>
        <code>{children}</code>
      </pre>
    ),
    { registerLanguage: vi.fn() },
  ),
}));

vi.mock('react-syntax-highlighter/dist/esm/styles/prism/one-dark', () => ({ default: {} }));
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

describe('CodeBlock', () => {
  afterEach(() => cleanup());

  it('renders code block container', () => {
    render(<CodeBlock language="typescript" code="const x = 1;" />);
    expect(screen.getByTestId('code-block')).toBeTruthy();
  });

  it('renders copy button', () => {
    render(<CodeBlock language="typescript" code="const x = 1;" />);
    expect(screen.getByTestId('code-copy-button')).toBeTruthy();
    expect(screen.getByText('Copy')).toBeTruthy();
  });

  it('renders language label', () => {
    render(<CodeBlock language="typescript" code="const x = 1;" />);
    expect(screen.getByTestId('code-language-label')).toBeTruthy();
    expect(screen.getByText('TypeScript')).toBeTruthy();
  });

  it('renders code content', () => {
    render(<CodeBlock language="go" code="func main() {}" />);
    expect(screen.getByText('func main() {}')).toBeTruthy();
  });

  it('shows Go label for go language', () => {
    render(<CodeBlock language="go" code="package main" />);
    expect(screen.getByText('Go')).toBeTruthy();
  });

  it('shows Python label for py alias', () => {
    render(<CodeBlock language="py" code="print(1)" />);
    expect(screen.getByText('Python')).toBeTruthy();
  });

  it('shows JavaScript label for js alias', () => {
    render(<CodeBlock language="js" code="let x;" />);
    expect(screen.getByText('JavaScript')).toBeTruthy();
  });

  it('shows Bash label for shell alias', () => {
    render(<CodeBlock language="shell" code="ls -la" />);
    expect(screen.getByText('Shell')).toBeTruthy();
  });

  it('shows Code for unknown language', () => {
    render(<CodeBlock language="" code="something" />);
    expect(screen.getByText('Code')).toBeTruthy();
  });

  it('capitalizes unknown language names', () => {
    render(<CodeBlock language="toml" code="[package]" />);
    expect(screen.getByText('Toml')).toBeTruthy();
  });

  it('shows line numbers for code > 5 lines', () => {
    const code = Array.from({ length: 10 }, (_, i) => `line ${i + 1}`).join('\n');
    render(<CodeBlock language="typescript" code={code} />);
    const highlighter = screen.getByTestId('mock-syntax-highlighter');
    expect(highlighter.getAttribute('data-line-numbers')).toBe('true');
  });

  it('hides line numbers for code <= 5 lines', () => {
    render(<CodeBlock language="typescript" code="const x = 1;" />);
    const highlighter = screen.getByTestId('mock-syntax-highlighter');
    expect(highlighter.getAttribute('data-line-numbers')).toBe('false');
  });

  it('changes to Copied! after click', () => {
    // Mock navigator.clipboard
    Object.assign(navigator, { clipboard: { writeText: vi.fn().mockResolvedValue(undefined) } });
    render(<CodeBlock language="typescript" code="const x = 1;" />);
    fireEvent.click(screen.getByTestId('code-copy-button'));
    expect(screen.getByText('Copied!')).toBeTruthy();
  });
});
