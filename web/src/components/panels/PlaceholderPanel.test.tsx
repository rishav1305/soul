// @vitest-environment happy-dom
import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import PlaceholderPanel from './PlaceholderPanel';

describe('PlaceholderPanel', () => {
  it('renders nothing', () => {
    const { container } = render(<PlaceholderPanel />);
    expect(container.innerHTML).toBe('');
  });
});
