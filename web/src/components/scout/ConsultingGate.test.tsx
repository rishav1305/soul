// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act, waitFor } from '@testing-library/react';
import { ConsultingGate } from './ConsultingGate';

const mockPost = vi.fn().mockResolvedValue({
  company_background: 'A tech company',
  likely_questions: ['Q1', 'Q2'],
  relevant_experience: ['Exp1'],
});

vi.mock('../../lib/api', () => ({
  api: {
    post: (...args: unknown[]) => mockPost(...args),
  },
}));

function makeLead(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'AI Strategy Consulting',
    company: 'ConsultCo',
    type: 'consulting' as const,
    source: 'referral',
    stage: 'screening',
    match_score: 80,
    compensation: '$300/hr',
    contact: '',
    location: 'Remote',
    notes: 'Need AI strategy review',
    url: '',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

describe('ConsultingGate', () => {
  afterEach(() => {
    cleanup();
    mockPost.mockClear();
  });

  it('renders consulting gate container', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('consulting-gate')).toBeTruthy();
  });

  it('shows company in header', () => {
    render(<ConsultingGate lead={makeLead({ company: 'TechCo' })} onAction={vi.fn()} />);
    expect(screen.getByText('TechCo')).toBeTruthy();
  });

  it('shows notes (or title as fallback)', () => {
    render(<ConsultingGate lead={makeLead({ notes: 'Need ML pipeline' })} onAction={vi.fn()} />);
    expect(screen.getByText('Need ML pipeline')).toBeTruthy();
  });

  it('shows title when notes empty', () => {
    render(<ConsultingGate lead={makeLead({ notes: '', title: 'AI Review' })} onAction={vi.fn()} />);
    expect(screen.getByText('AI Review')).toBeTruthy();
  });

  it('shows stage badge', () => {
    render(<ConsultingGate lead={makeLead({ stage: 'interviewing' })} onAction={vi.fn()} />);
    expect(screen.getByText('interviewing')).toBeTruthy();
  });

  it('renders all 4 section toggles', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('consulting-section-call-prep')).toBeTruthy();
    expect(screen.getByTestId('consulting-section-advisory')).toBeTruthy();
    expect(screen.getByTestId('consulting-section-project')).toBeTruthy();
    expect(screen.getByTestId('consulting-section-upsell')).toBeTruthy();
  });

  it('shows section labels', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByText('Call Prep')).toBeTruthy();
    expect(screen.getByText('Advisory Proposal')).toBeTruthy();
    expect(screen.getByText('Project Proposal')).toBeTruthy();
    expect(screen.getByText('Upsell Evaluation')).toBeTruthy();
  });

  it('call-prep section expanded by default', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('consulting-generate-call-prep-btn')).toBeTruthy();
  });

  it('collapses section on toggle click', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('consulting-section-call-prep'));
    expect(screen.queryByTestId('consulting-generate-call-prep-btn')).toBeNull();
  });

  it('expands collapsed section on click', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('consulting-section-advisory'));
    expect(screen.getByTestId('consulting-generate-advisory-btn')).toBeTruthy();
  });

  it('renders footer action buttons', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('consulting-send-proposal-btn')).toBeTruthy();
    expect(screen.getByTestId('consulting-follow-up-btn')).toBeTruthy();
    expect(screen.getByTestId('consulting-skip-btn')).toBeTruthy();
  });

  it('calls onAction with send-proposal', () => {
    const onAction = vi.fn();
    render(<ConsultingGate lead={makeLead({ id: 15 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('consulting-send-proposal-btn'));
    expect(onAction).toHaveBeenCalledWith('send-proposal', 15);
  });

  it('calls onAction with follow-up', () => {
    const onAction = vi.fn();
    render(<ConsultingGate lead={makeLead({ id: 15 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('consulting-follow-up-btn'));
    expect(onAction).toHaveBeenCalledWith('follow-up', 15);
  });

  it('calls onAction with skip', () => {
    const onAction = vi.fn();
    render(<ConsultingGate lead={makeLead({ id: 15 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('consulting-skip-btn'));
    expect(onAction).toHaveBeenCalledWith('skip', 15);
  });

  it('shows button labels', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByText('Send Proposal')).toBeTruthy();
    expect(screen.getByText('Skip')).toBeTruthy();
  });

  it('generate call-prep calls API with lead_id', async () => {
    mockPost.mockResolvedValueOnce({
      company_background: 'Tech firm',
      likely_questions: ['Q1'],
      relevant_experience: ['Exp1'],
    });
    render(<ConsultingGate lead={makeLead({ id: 25 })} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('consulting-generate-call-prep-btn'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/call-prep', { lead_id: 25 });
  });

  it('displays call-prep result with all fields', async () => {
    mockPost.mockResolvedValueOnce({
      company_background: 'Leading AI company',
      likely_questions: ['What is your approach?', 'Timeline?'],
      relevant_experience: ['Built ML systems', 'Consulted for 5 firms'],
    });
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('consulting-generate-call-prep-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('consulting-call-prep-result')).toBeTruthy();
    });
    expect(screen.getByText('Leading AI company')).toBeTruthy();
    expect(screen.getByText('What is your approach?')).toBeTruthy();
    expect(screen.getByText('Built ML systems')).toBeTruthy();
  });

  it('hides generate button after successful call-prep', async () => {
    mockPost.mockResolvedValueOnce({
      company_background: 'Co',
      likely_questions: [],
      relevant_experience: [],
    });
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('consulting-generate-call-prep-btn'));
    });
    await waitFor(() => {
      expect(screen.queryByTestId('consulting-generate-call-prep-btn')).toBeNull();
    });
  });

  it('generate advisory calls correct API endpoint', async () => {
    mockPost.mockResolvedValueOnce({
      executive_summary: 'Summary',
      scope: 'Full scope',
      deliverables: ['Report'],
      pricing: '$10k',
    });
    render(<ConsultingGate lead={makeLead({ id: 30 })} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('consulting-section-advisory'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('consulting-generate-advisory-btn'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/advisory-proposal', { lead_id: 30 });
  });

  it('displays advisory result with executive summary and deliverables', async () => {
    mockPost.mockResolvedValueOnce({
      executive_summary: 'AI transformation strategy',
      scope: 'Q2 2026',
      deliverables: ['Architecture review', 'Implementation plan'],
      pricing: '$15k/month',
    });
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('consulting-section-advisory'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('consulting-generate-advisory-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('consulting-advisory-result')).toBeTruthy();
    });
    expect(screen.getByText('AI transformation strategy')).toBeTruthy();
    expect(screen.getByText('Architecture review')).toBeTruthy();
    expect(screen.getByText('$15k/month')).toBeTruthy();
  });

  it('generate project calls correct API endpoint', async () => {
    mockPost.mockResolvedValueOnce({
      milestones: [{ name: 'Phase 1', duration: '2 weeks', description: 'Setup' }],
      budget: '$50k',
      timeline: '3 months',
    });
    render(<ConsultingGate lead={makeLead({ id: 40 })} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('consulting-section-project'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('consulting-generate-project-btn'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/project-proposal', { lead_id: 40 });
  });

  it('displays project result with milestones, budget, timeline', async () => {
    mockPost.mockResolvedValueOnce({
      milestones: [
        { name: 'Discovery', duration: '1 week', description: 'Requirements gathering' },
        { name: 'Build', duration: '4 weeks', description: 'Implementation' },
      ],
      budget: '$75k',
      timeline: '6 weeks',
    });
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('consulting-section-project'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('consulting-generate-project-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('consulting-project-result')).toBeTruthy();
    });
    expect(screen.getByText('Discovery')).toBeTruthy();
    expect(screen.getByText('Build')).toBeTruthy();
    expect(screen.getByText('$75k')).toBeTruthy();
    expect(screen.getByText('6 weeks')).toBeTruthy();
  });

  it('generate upsell calls correct API endpoint', async () => {
    mockPost.mockResolvedValueOnce({
      score: 85,
      opportunities: ['Expand to ML ops'],
    });
    render(<ConsultingGate lead={makeLead({ id: 50 })} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('consulting-section-upsell'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('consulting-generate-upsell-btn'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/consulting-upsell', { lead_id: 50 });
  });

  it('displays upsell result with score and opportunities', async () => {
    mockPost.mockResolvedValueOnce({
      score: 90,
      opportunities: ['Cloud migration', 'Team training'],
    });
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('consulting-section-upsell'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('consulting-generate-upsell-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('consulting-upsell-result')).toBeTruthy();
    });
    expect(screen.getByText('90')).toBeTruthy();
    expect(screen.getByText('Cloud migration')).toBeTruthy();
    expect(screen.getByText('Team training')).toBeTruthy();
  });

  it('upsell score shows High potential for score >= 80', async () => {
    mockPost.mockResolvedValueOnce({ score: 85, opportunities: [] });
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('consulting-section-upsell'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('consulting-generate-upsell-btn'));
    });
    await waitFor(() => {
      expect(screen.getByText('High potential')).toBeTruthy();
    });
  });

  it('upsell score shows Moderate potential for score 50-79', async () => {
    mockPost.mockResolvedValueOnce({ score: 60, opportunities: [] });
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('consulting-section-upsell'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('consulting-generate-upsell-btn'));
    });
    await waitFor(() => {
      expect(screen.getByText('Moderate potential')).toBeTruthy();
    });
  });

  it('upsell score shows Low potential for score < 50', async () => {
    mockPost.mockResolvedValueOnce({ score: 30, opportunities: [] });
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('consulting-section-upsell'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('consulting-generate-upsell-btn'));
    });
    await waitFor(() => {
      expect(screen.getByText('Low potential')).toBeTruthy();
    });
  });

  it('shows error when API call fails', async () => {
    mockPost.mockRejectedValueOnce(new Error('Network timeout'));
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('consulting-generate-call-prep-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('consulting-gate-error')).toBeTruthy();
      expect(screen.getByText('Network timeout')).toBeTruthy();
    });
  });

  it('shows Generating... during call-prep loading', async () => {
    let resolvePost: (val: unknown) => void;
    mockPost.mockImplementationOnce(() => new Promise(r => { resolvePost = r; }));
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('consulting-generate-call-prep-btn'));
    });
    expect(screen.getByText('Generating...')).toBeTruthy();
    await act(async () => {
      resolvePost!({ company_background: '', likely_questions: [], relevant_experience: [] });
    });
  });

  it('multiple sections can be expanded simultaneously', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    // call-prep expanded by default
    expect(screen.getByTestId('consulting-generate-call-prep-btn')).toBeTruthy();
    // expand advisory
    fireEvent.click(screen.getByTestId('consulting-section-advisory'));
    expect(screen.getByTestId('consulting-generate-advisory-btn')).toBeTruthy();
    // call-prep should still be expanded
    expect(screen.getByTestId('consulting-generate-call-prep-btn')).toBeTruthy();
  });

  it('shows Collapse/Expand text in toggle buttons', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    // call-prep is expanded → Collapse
    expect(screen.getByTestId('consulting-section-call-prep').textContent).toContain('Collapse');
    // advisory is collapsed → Expand
    expect(screen.getByTestId('consulting-section-advisory').textContent).toContain('Expand');
  });

  it('shows Follow Up button text', () => {
    render(<ConsultingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByText('Follow Up')).toBeTruthy();
  });
});
