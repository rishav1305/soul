// @vitest-environment jsdom
import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import ScoutPanel from './ScoutPanel';

describe('ScoutPanel', () => {
  it('renders nothing (stub)', () => {
    const { container } = render(<ScoutPanel />);
    expect(container.innerHTML).toBe('');
  });
});
