// @vitest-environment jsdom
import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import TutorPanel from './TutorPanel';

describe('TutorPanel', () => {
  it('renders nothing (stub)', () => {
    const { container } = render(<TutorPanel />);
    expect(container.innerHTML).toBe('');
  });
});
