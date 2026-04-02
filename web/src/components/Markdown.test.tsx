// @vitest-environment happy-dom
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

  it('renders h2 headings', () => {
    render(<Markdown content="## Heading 2" />);
    const h2 = screen.getByText('Heading 2');
    expect(h2.tagName).toBe('H2');
  });

  it('renders h3 headings', () => {
    render(<Markdown content="### Heading 3" />);
    const h3 = screen.getByText('Heading 3');
    expect(h3.tagName).toBe('H3');
  });

  it('renders ordered lists', () => {
    render(<Markdown content={"1. First\n2. Second\n3. Third"} />);
    expect(screen.getByText('First')).toBeTruthy();
    expect(screen.getByText('Second')).toBeTruthy();
    expect(screen.getByText('Third')).toBeTruthy();
  });

  it('renders horizontal rules', () => {
    const { container } = render(<Markdown content={"Above\n\n---\n\nBelow"} />);
    const hr = container.querySelector('hr');
    expect(hr).toBeTruthy();
  });

  it('renders GFM tables', () => {
    const tableContent = "| Name | Age |\n| --- | --- |\n| Alice | 30 |";
    const { container } = render(<Markdown content={tableContent} />);
    expect(container.querySelector('table')).toBeTruthy();
    expect(container.querySelector('thead')).toBeTruthy();
    expect(screen.getByText('Name')).toBeTruthy();
    expect(screen.getByText('Alice')).toBeTruthy();
  });

  it('renders code block without language', () => {
    render(<Markdown content={"```\nplain text\n```"} />);
    const codeBlock = screen.getByTestId('mock-code-block');
    expect(codeBlock).toBeTruthy();
    expect(codeBlock.getAttribute('data-language')).toBe('');
  });

  it('renders multiple paragraphs', () => {
    render(<Markdown content={"First paragraph\n\nSecond paragraph"} />);
    expect(screen.getByText('First paragraph')).toBeTruthy();
    expect(screen.getByText('Second paragraph')).toBeTruthy();
  });

  it('renders strikethrough text with GFM', () => {
    render(<Markdown content="~~strikethrough~~" />);
    const del = screen.getByText('strikethrough');
    expect(del.tagName).toBe('DEL');
  });

  it('renders nested inline elements', () => {
    render(<Markdown content="**bold and *italic***" />);
    const container = screen.getByTestId('markdown-content');
    expect(container.querySelector('strong')).toBeTruthy();
    expect(container.querySelector('em')).toBeTruthy();
  });

  it('links have noopener noreferrer for security', () => {
    render(<Markdown content="[Link](https://example.com)" />);
    const link = screen.getByText('Link') as HTMLAnchorElement;
    expect(link.getAttribute('rel')).toContain('noreferrer');
  });
});
