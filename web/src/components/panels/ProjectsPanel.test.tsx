// @vitest-environment jsdom
import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import ProjectsPanel from './ProjectsPanel';

describe('ProjectsPanel', () => {
  it('renders nothing (stub)', () => {
    const { container } = render(<ProjectsPanel />);
    expect(container.innerHTML).toBe('');
  });
});
