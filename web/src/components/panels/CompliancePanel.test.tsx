// @vitest-environment jsdom
import { describe, it, expect } from 'vitest';
import { render } from '@testing-library/react';
import CompliancePanel from './CompliancePanel';

describe('CompliancePanel', () => {
  it('renders nothing (stub)', () => {
    const { container } = render(<CompliancePanel />);
    expect(container.innerHTML).toBe('');
  });
});
