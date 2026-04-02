// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { PipelineBoard } from './PipelineBoard';

vi.mock('./LeadCard', () => ({
  LeadCard: ({ lead, onClick }: { lead: { id: number; title: string }; onClick: (l: unknown) => void }) => (
    <div data-testid={`mock-lead-card-${lead.id}`} onClick={() => onClick(lead)}>{lead.title}</div>
  ),
}));

function makeLead(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'Engineer',
    company: 'Acme',
    type: 'job' as const,
    source: 'linkedin',
    stage: 'discovered',
    match_score: 70,
    compensation: '',
    contact: '',
    location: '',
    notes: '',
    url: '',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

describe('PipelineBoard', () => {
  afterEach(() => cleanup());

  it('renders pipeline board container', () => {
    render(<PipelineBoard leads={[]} onSelectLead={vi.fn()} />);
    expect(screen.getByTestId('pipeline-board')).toBeTruthy();
  });

  it('renders type filter buttons', () => {
    render(<PipelineBoard leads={[]} onSelectLead={vi.fn()} />);
    expect(screen.getByTestId('pipeline-type-filter')).toBeTruthy();
    expect(screen.getByTestId('filter-type-all')).toBeTruthy();
    expect(screen.getByTestId('filter-type-job')).toBeTruthy();
    expect(screen.getByTestId('filter-type-freelance')).toBeTruthy();
    expect(screen.getByTestId('filter-type-contract')).toBeTruthy();
    expect(screen.getByTestId('filter-type-consulting')).toBeTruthy();
  });

  it('renders all 6 pipeline columns', () => {
    render(<PipelineBoard leads={[]} onSelectLead={vi.fn()} />);
    expect(screen.getByTestId('pipeline-column-discovered')).toBeTruthy();
    expect(screen.getByTestId('pipeline-column-applied')).toBeTruthy();
    expect(screen.getByTestId('pipeline-column-screening')).toBeTruthy();
    expect(screen.getByTestId('pipeline-column-interviewing')).toBeTruthy();
    expect(screen.getByTestId('pipeline-column-negotiating')).toBeTruthy();
    expect(screen.getByTestId('pipeline-column-closed')).toBeTruthy();
  });

  it('shows column labels', () => {
    render(<PipelineBoard leads={[]} onSelectLead={vi.fn()} />);
    expect(screen.getByText('Discovered')).toBeTruthy();
    expect(screen.getByText('Applied')).toBeTruthy();
    expect(screen.getByText('Screening')).toBeTruthy();
  });

  it('renders leads in correct columns', () => {
    const leads = [
      makeLead({ id: 1, stage: 'discovered', title: 'Lead A' }),
      makeLead({ id: 2, stage: 'screening', title: 'Lead B' }),
    ];
    render(<PipelineBoard leads={leads} onSelectLead={vi.fn()} />);
    expect(screen.getByTestId('mock-lead-card-1')).toBeTruthy();
    expect(screen.getByTestId('mock-lead-card-2')).toBeTruthy();
  });

  it('filters by type', () => {
    const leads = [
      makeLead({ id: 1, type: 'job', stage: 'discovered' }),
      makeLead({ id: 2, type: 'freelance', stage: 'discovered' }),
    ];
    render(<PipelineBoard leads={leads} onSelectLead={vi.fn()} />);
    fireEvent.click(screen.getByTestId('filter-type-job'));
    expect(screen.getByTestId('mock-lead-card-1')).toBeTruthy();
    expect(screen.queryByTestId('mock-lead-card-2')).toBeNull();
  });

  it('shows all when All filter selected', () => {
    const leads = [
      makeLead({ id: 1, type: 'job', stage: 'discovered' }),
      makeLead({ id: 2, type: 'freelance', stage: 'discovered' }),
    ];
    render(<PipelineBoard leads={leads} onSelectLead={vi.fn()} />);
    fireEvent.click(screen.getByTestId('filter-type-job'));
    fireEvent.click(screen.getByTestId('filter-type-all'));
    expect(screen.getByTestId('mock-lead-card-1')).toBeTruthy();
    expect(screen.getByTestId('mock-lead-card-2')).toBeTruthy();
  });

  it('calls onSelectLead when card clicked', () => {
    const onSelect = vi.fn();
    const leads = [makeLead({ id: 5, stage: 'discovered' })];
    render(<PipelineBoard leads={leads} onSelectLead={onSelect} />);
    fireEvent.click(screen.getByTestId('mock-lead-card-5'));
    expect(onSelect).toHaveBeenCalled();
  });

  it('shows lead count in column header', () => {
    const leads = [
      makeLead({ id: 1, stage: 'discovered' }),
      makeLead({ id: 2, stage: 'discovered' }),
    ];
    render(<PipelineBoard leads={leads} onSelectLead={vi.fn()} />);
    const col = screen.getByTestId('pipeline-column-discovered');
    expect(col.textContent).toContain('2');
  });
});
