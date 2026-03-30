// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { Markdown } from './Markdown';

// Mock CodeBlock to avoid syntax highlighter complexity
vi.mock('./CodeBlock', () => ({
  default: ({ language, code }: { language: string; code: string }) => (
    <div data-testid="mock-code-block" data-language={language}>
      {code}
    </div>
  ),
}));

describe('Markdown', () => {
  afterEach(() => cleanup());

  it('returns null for empty content', () => {
    const { container } = render(<Markdown content="" />);
    expect(container.innerHTML).toBe('');
  });

  it('renders markdown content container', () => {
    render(<Markdown content="Hello" />);
    expect(screen.getByTestId('markdown-content')).toBeTruthy();
  });

  it('renders plain text', () => {
    render(<Markdown content="Hello world" />);
    expect(screen.getByText('Hello world')).toBeTruthy();
  });

  it('renders bold text', () => {
    render(<Markdown content="**bold text**" />);
    const strong = screen.getByText('bold text');
    expect(strong.tagName).toBe('STRONG');
  });

  it('renders italic text', () => {
    render(<Markdown content="*italic text*" />);
    const em = screen.getByText('italic text');
    expect(em.tagName).toBe('EM');
  });

  it('renders headings', () => {
    render(<Markdown content="# Heading 1" />);
    const h1 = screen.getByText('Heading 1');
    expect(h1.tagName).toBe('H1');
  });

  it('renders links with target blank', () => {
    render(<Markdown content="[Link](https://example.com)" />);
    const link = screen.getByText('Link') as HTMLAnchorElement;
    expect(link.tagName).toBe('A');
    expect(link.getAttribute('target')).toBe('_blank');
    expect(link.getAttribute('rel')).toContain('noopener');
    expect(link.getAttribute('href')).toBe('https://example.com');
  });

  it('renders unordered lists', () => {
    render(<Markdown content={"- Item 1\n- Item 2"} />);
    expect(screen.getByText('Item 1')).toBeTruthy();
    expect(screen.getByText('Item 2')).toBeTruthy();
  });

  it('renders code blocks via CodeBlock component', () => {
    render(<Markdown content={"```typescript\nconst x = 1;\n```"} />);
    const codeBlock = screen.getByTestId('mock-code-block');
    expect(codeBlock).toBeTruthy();
    expect(codeBlock.getAttribute('data-language')).toBe('typescript');
  });

  it('renders inline code', () => {
    render(<Markdown content="Use `npm install` to start" />);
    const code = screen.getByText('npm install');
    expect(code.tagName).toBe('CODE');
  });

  it('renders blockquotes', () => {
    render(<Markdown content="> This is a quote" />);
    const quote = screen.getByText('This is a quote');
    expect(quote.closest('blockquote')).toBeTruthy();
  });

  it('renders images with lazy loading', () => {
    render(<Markdown content="![Alt text](https://example.com/img.png)" />);
    const img = screen.getByAltText('Alt text') as HTMLImageElement;
    expect(img.getAttribute('loading')).toBe('lazy');
    expect(img.getAttribute('src')).toBe('https://example.com/img.png');
  });
});
