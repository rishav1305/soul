// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import { LeadDetail } from './LeadDetail';

function makeLead(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'Senior Engineer',
    company: 'Acme Corp',
    type: 'job' as const,
    source: 'linkedin',
    stage: 'screening',
    match_score: 80,
    compensation: '200k',
    contact: 'Jane HR',
    location: 'Remote',
    notes: 'Strong candidate',
    url: '',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

describe('LeadDetail', () => {
  afterEach(() => cleanup());

  it('renders lead detail container', () => {
    render(<LeadDetail lead={makeLead()} onUpdate={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByTestId('lead-detail')).toBeTruthy();
  });

  it('shows lead title', () => {
    render(<LeadDetail lead={makeLead({ title: 'ML Engineer' })} onUpdate={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText('ML Engineer')).toBeTruthy();
  });

  it('renders close button', () => {
    render(<LeadDetail lead={makeLead()} onUpdate={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByTestId('lead-detail-close')).toBeTruthy();
  });

  it('calls onClose when close clicked', () => {
    const onClose = vi.fn();
    render(<LeadDetail lead={makeLead()} onUpdate={vi.fn()} onClose={onClose} />);
    fireEvent.click(screen.getByTestId('lead-detail-close'));
    expect(onClose).toHaveBeenCalled();
  });

  it('shows company', () => {
    render(<LeadDetail lead={makeLead({ company: 'BigCo' })} onUpdate={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText('BigCo')).toBeTruthy();
  });

  it('shows type', () => {
    render(<LeadDetail lead={makeLead({ type: 'freelance' })} onUpdate={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText('freelance')).toBeTruthy();
  });

  it('shows source', () => {
    render(<LeadDetail lead={makeLead({ source: 'referral' })} onUpdate={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText('referral')).toBeTruthy();
  });

  it('shows compensation', () => {
    render(<LeadDetail lead={makeLead({ compensation: '150k' })} onUpdate={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText('150k')).toBeTruthy();
  });

  it('shows notes', () => {
    render(<LeadDetail lead={makeLead({ notes: 'Great fit' })} onUpdate={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText('Great fit')).toBeTruthy();
  });

  it('shows No notes when empty', () => {
    render(<LeadDetail lead={makeLead({ notes: '' })} onUpdate={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText('No notes')).toBeTruthy();
  });

  it('shows Edit button for notes', () => {
    render(<LeadDetail lead={makeLead()} onUpdate={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByTestId('lead-detail-edit-notes')).toBeTruthy();
    expect(screen.getByText('Edit')).toBeTruthy();
  });

  it('shows notes input when Edit clicked', () => {
    render(<LeadDetail lead={makeLead()} onUpdate={vi.fn()} onClose={vi.fn()} />);
    fireEvent.click(screen.getByTestId('lead-detail-edit-notes'));
    expect(screen.getByTestId('lead-detail-notes-input')).toBeTruthy();
    expect(screen.getByTestId('lead-detail-save-notes')).toBeTruthy();
  });

  it('calls onUpdate with notes when Save clicked', async () => {
    const onUpdate = vi.fn().mockResolvedValue(undefined);
    render(<LeadDetail lead={makeLead({ id: 5 })} onUpdate={onUpdate} onClose={vi.fn()} />);
    fireEvent.click(screen.getByTestId('lead-detail-edit-notes'));
    fireEvent.change(screen.getByTestId('lead-detail-notes-input'), { target: { value: 'Updated' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('lead-detail-save-notes'));
    });
    expect(onUpdate).toHaveBeenCalledWith(5, { notes: 'Updated' });
  });

  it('shows current stage', () => {
    render(<LeadDetail lead={makeLead({ stage: 'interviewing' })} onUpdate={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByText('interviewing')).toBeTruthy();
  });

  it('shows advance button with next stage', () => {
    render(<LeadDetail lead={makeLead({ stage: 'screening' })} onUpdate={vi.fn()} onClose={vi.fn()} />);
    expect(screen.getByTestId('lead-detail-advance')).toBeTruthy();
    expect(screen.getByText('Advance to interviewing')).toBeTruthy();
  });

  it('hides advance when at last stage', () => {
    render(<LeadDetail lead={makeLead({ stage: 'closed' })} onUpdate={vi.fn()} onClose={vi.fn()} />);
    expect(screen.queryByTestId('lead-detail-advance')).toBeNull();
  });

  it('calls onUpdate with next stage on advance', async () => {
    const onUpdate = vi.fn().mockResolvedValue(undefined);
    render(<LeadDetail lead={makeLead({ id: 3, stage: 'applied' })} onUpdate={onUpdate} onClose={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('lead-detail-advance'));
    });
    expect(onUpdate).toHaveBeenCalledWith(3, { stage: 'screening' });
  });
});
