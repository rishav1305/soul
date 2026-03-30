// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { DiffBlock } from './DiffBlock';

afterEach(() => cleanup());

describe('DiffBlock', () => {
  it('renders diff content', () => {
    render(<DiffBlock content="+ added line" />);
    const block = screen.getByTestId('diff-block');
    expect(block).toBeTruthy();
    expect(block.textContent).toContain('+ added line');
  });

  it('applies green styling for additions', () => {
    const { container } = render(<DiffBlock content="+added" />);
    const line = container.querySelector('.text-green-400');
    expect(line).toBeTruthy();
    expect(line!.textContent).toBe('+added');
  });

  it('applies red styling for deletions', () => {
    const { container } = render(<DiffBlock content="-removed" />);
    const line = container.querySelector('.text-red-400');
    expect(line).toBeTruthy();
    expect(line!.textContent).toBe('-removed');
  });

  it('applies soul styling for hunk headers', () => {
    const { container } = render(<DiffBlock content="@@ -1,3 +1,4 @@" />);
    const line = container.querySelector('[class*="text-soul"]');
    expect(line).toBeTruthy();
  });

  it('renders empty lines as non-breaking spaces', () => {
    const { container } = render(<DiffBlock content={"line1\n\nline3"} />);
    const divs = container.querySelectorAll('pre > div');
    expect(divs.length).toBe(3);
    expect(divs[1].textContent).toBe('\u00a0');
  });

  it('handles multi-line diff content', () => {
    const diff = `@@ -1,3 +1,4 @@
 unchanged
-old line
+new line
+added line`;
    render(<DiffBlock content={diff} />);
    const block = screen.getByTestId('diff-block');
    const lines = block.querySelectorAll('div');
    expect(lines.length).toBe(5);
  });
});
