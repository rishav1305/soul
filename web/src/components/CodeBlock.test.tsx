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

  it('uses fallback copy when clipboard API unavailable', () => {
    // Remove clipboard API
    Object.assign(navigator, { clipboard: undefined });
    // jsdom doesn't define execCommand — add it
    document.execCommand = vi.fn().mockReturnValue(true);
    render(<CodeBlock language="go" code="package main" />);
    fireEvent.click(screen.getByTestId('code-copy-button'));
    expect(document.execCommand).toHaveBeenCalledWith('copy');
    expect(screen.getByText('Copied!')).toBeTruthy();
  });

  it('shows YAML label for yml alias', () => {
    render(<CodeBlock language="yml" code="key: value" />);
    expect(screen.getByText('YAML')).toBeTruthy();
  });

  it('shows Markdown label for md alias', () => {
    render(<CodeBlock language="md" code="# Hello" />);
    expect(screen.getByText('Markdown')).toBeTruthy();
  });

  it('shows Dockerfile label for dockerfile alias', () => {
    render(<CodeBlock language="dockerfile" code="FROM node:20" />);
    expect(screen.getByText('Dockerfile')).toBeTruthy();
  });

  it('shows JSX label for jsx alias', () => {
    render(<CodeBlock language="jsx" code="<App />" />);
    expect(screen.getByText('JSX')).toBeTruthy();
  });

  it('shows Rust label for rs alias', () => {
    render(<CodeBlock language="rs" code="fn main() {}" />);
    expect(screen.getByText('Rust')).toBeTruthy();
  });

  it('shows Go label for golang alias', () => {
    render(<CodeBlock language="golang" code="func main() {}" />);
    expect(screen.getByText('Go')).toBeTruthy();
  });

  it('hides line numbers at exactly 5 lines', () => {
    const code = 'a\nb\nc\nd\ne';
    render(<CodeBlock language="go" code={code} />);
    const h = screen.getByTestId('mock-syntax-highlighter');
    expect(h.getAttribute('data-line-numbers')).toBe('false');
  });

  it('shows line numbers at exactly 6 lines', () => {
    const code = 'a\nb\nc\nd\ne\nf';
    render(<CodeBlock language="go" code={code} />);
    const h = screen.getByTestId('mock-syntax-highlighter');
    expect(h.getAttribute('data-line-numbers')).toBe('true');
  });

  it('shows SQL label', () => {
    render(<CodeBlock language="sql" code="SELECT 1" />);
    expect(screen.getByText('SQL')).toBeTruthy();
  });

  it('shows CSS label', () => {
    render(<CodeBlock language="css" code="body { color: red; }" />);
    expect(screen.getByText('CSS')).toBeTruthy();
  });

  it('shows Java label', () => {
    render(<CodeBlock language="java" code="class Main {}" />);
    expect(screen.getByText('Java')).toBeTruthy();
  });

  it('passes lowercase language to highlighter', () => {
    render(<CodeBlock language="TypeScript" code="const x = 1;" />);
    const h = screen.getByTestId('mock-syntax-highlighter');
    expect(h.getAttribute('data-language')).toBe('typescript');
  });
});
