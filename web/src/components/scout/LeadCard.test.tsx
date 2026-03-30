// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { LeadCard } from './LeadCard';

function makeLead(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'Senior Go Engineer',
    company: 'Acme Corp',
    stage: 'discovered',
    source: 'linkedin',
    match_score: 85,
    pipeline: 'job',
    priority: 'medium',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    ...overrides,
  };
}

describe('LeadCard', () => {
  afterEach(() => cleanup());

  it('renders lead card with testid', () => {
    render(<LeadCard lead={makeLead({ id: 42 })} onClick={vi.fn()} />);
    expect(screen.getByTestId('lead-card-42')).toBeTruthy();
  });

  it('shows lead title', () => {
    render(<LeadCard lead={makeLead({ title: 'React Developer' })} onClick={vi.fn()} />);
    expect(screen.getByText('React Developer')).toBeTruthy();
  });

  it('shows company name', () => {
    render(<LeadCard lead={makeLead({ company: 'Google' })} onClick={vi.fn()} />);
    expect(screen.getByText('Google')).toBeTruthy();
  });

  it('shows stage label', () => {
    render(<LeadCard lead={makeLead({ stage: 'interviewing' })} onClick={vi.fn()} />);
    expect(screen.getByText('interviewing')).toBeTruthy();
  });

  it('shows source', () => {
    render(<LeadCard lead={makeLead({ source: 'github' })} onClick={vi.fn()} />);
    expect(screen.getByText('github')).toBeTruthy();
  });

  it('shows match score with percentage', () => {
    render(<LeadCard lead={makeLead({ match_score: 92 })} onClick={vi.fn()} />);
    expect(screen.getByText('92%')).toBeTruthy();
  });

  it('calls onClick with lead when clicked', () => {
    const onClick = vi.fn();
    const lead = makeLead({ id: 5 });
    render(<LeadCard lead={lead} onClick={onClick} />);
    fireEvent.click(screen.getByTestId('lead-card-5'));
    expect(onClick).toHaveBeenCalledWith(lead);
  });

  it('renders as button element', () => {
    render(<LeadCard lead={makeLead()} onClick={vi.fn()} />);
    expect(screen.getByTestId('lead-card-1').tagName).toBe('BUTTON');
  });

  it('hides company when not set', () => {
    render(<LeadCard lead={makeLead({ company: '' })} onClick={vi.fn()} />);
    // No company element should be rendered
    const card = screen.getByTestId('lead-card-1');
    expect(card.textContent).not.toContain('Acme');
  });
});
