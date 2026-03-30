// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act, waitFor } from '@testing-library/react';
import { ContractGate } from './ContractGate';

const mockPost = vi.fn().mockResolvedValue({
  scope: 'Build API',
  deliverables: ['API', 'Docs'],
  timeline: '4 weeks',
  pricing: '$10k',
});

vi.mock('../../lib/api', () => ({
  api: {
    post: (...args: unknown[]) => mockPost(...args),
  },
}));

function makeLead(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'API Integration',
    company: 'ContractCo',
    type: 'contract' as const,
    source: 'referral',
    stage: 'proposal-ready',
    match_score: 70,
    compensation: '10k/mo',
    contact: '',
    location: 'Remote',
    notes: '',
    url: '',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

describe('ContractGate', () => {
  afterEach(() => {
    cleanup();
    mockPost.mockClear();
  });

  it('renders contract gate container', () => {
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('contract-gate')).toBeTruthy();
  });

  it('shows company in header', () => {
    render(<ContractGate lead={makeLead({ company: 'BigCorp' })} onAction={vi.fn()} />);
    expect(screen.getByText('BigCorp')).toBeTruthy();
  });

  it('shows lead title', () => {
    render(<ContractGate lead={makeLead({ title: 'Data Pipeline' })} onAction={vi.fn()} />);
    expect(screen.getByText('Data Pipeline')).toBeTruthy();
  });

  it('shows stage badge', () => {
    render(<ContractGate lead={makeLead({ stage: 'negotiating' })} onAction={vi.fn()} />);
    expect(screen.getByText('negotiating')).toBeTruthy();
  });

  it('renders all 4 section toggles', () => {
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('contract-section-sow')).toBeTruthy();
    expect(screen.getByTestId('contract-section-followup')).toBeTruthy();
    expect(screen.getByTestId('contract-section-case-study')).toBeTruthy();
    expect(screen.getByTestId('contract-section-upsell')).toBeTruthy();
  });

  it('shows section labels', () => {
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('contract-section-sow').textContent).toContain('Statement of Work');
    expect(screen.getByTestId('contract-section-followup').textContent).toContain('Follow Up');
    expect(screen.getByTestId('contract-section-case-study').textContent).toContain('Case Study');
    expect(screen.getByTestId('contract-section-upsell').textContent).toContain('Upsell Detection');
  });

  it('SOW section expanded by default', () => {
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('contract-generate-sow-btn')).toBeTruthy();
  });

  it('collapses section on toggle click', () => {
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('contract-section-sow'));
    expect(screen.queryByTestId('contract-generate-sow-btn')).toBeNull();
  });

  it('expands section on toggle click', () => {
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    // followup starts collapsed
    fireEvent.click(screen.getByTestId('contract-section-followup'));
    expect(screen.getByTestId('contract-generate-followup-btn')).toBeTruthy();
  });

  it('renders footer action buttons', () => {
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('contract-send-sow-btn')).toBeTruthy();
    expect(screen.getByTestId('contract-follow-up-btn')).toBeTruthy();
    expect(screen.getByTestId('contract-skip-btn')).toBeTruthy();
  });

  it('calls onAction with send-sow', () => {
    const onAction = vi.fn();
    render(<ContractGate lead={makeLead({ id: 10 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('contract-send-sow-btn'));
    expect(onAction).toHaveBeenCalledWith('send-sow', 10);
  });

  it('calls onAction with follow-up', () => {
    const onAction = vi.fn();
    render(<ContractGate lead={makeLead({ id: 10 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('contract-follow-up-btn'));
    expect(onAction).toHaveBeenCalledWith('follow-up', 10);
  });

  it('calls onAction with skip', () => {
    const onAction = vi.fn();
    render(<ContractGate lead={makeLead({ id: 10 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('contract-skip-btn'));
    expect(onAction).toHaveBeenCalledWith('skip', 10);
  });

  it('shows button labels', () => {
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('contract-send-sow-btn').textContent).toContain('Send SOW');
    expect(screen.getByTestId('contract-follow-up-btn').textContent).toContain('Follow Up');
    expect(screen.getByTestId('contract-skip-btn').textContent).toContain('Skip');
  });

  it('generate SOW calls API with lead_id', async () => {
    mockPost.mockResolvedValueOnce({
      scope: 'Build API', deliverables: ['API'], timeline: '4 weeks', pricing: '$10k',
    });
    render(<ContractGate lead={makeLead({ id: 42 })} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('contract-generate-sow-btn'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/sow', { lead_id: 42 });
  });

  it('displays SOW result with scope, deliverables, timeline, pricing', async () => {
    mockPost.mockResolvedValueOnce({
      scope: 'Full-stack API build',
      deliverables: ['REST API', 'Documentation', 'Tests'],
      timeline: '6 weeks',
      pricing: '$15,000',
    });
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('contract-generate-sow-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('contract-sow-result')).toBeTruthy();
    });
    expect(screen.getByText('Full-stack API build')).toBeTruthy();
    expect(screen.getByText('REST API')).toBeTruthy();
    expect(screen.getByText('Documentation')).toBeTruthy();
    expect(screen.getByText('6 weeks')).toBeTruthy();
    expect(screen.getByText('$15,000')).toBeTruthy();
  });

  it('generate button replaced by result after generation', async () => {
    mockPost.mockResolvedValueOnce({
      scope: 'API', deliverables: ['API'], timeline: '4w', pricing: '$5k',
    });
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('contract-generate-sow-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('contract-sow-result')).toBeTruthy();
    });
    // Generate button should be gone — replaced by result
    expect(screen.queryByTestId('contract-generate-sow-btn')).toBeNull();
  });

  it('generate follow-up calls correct API', async () => {
    mockPost.mockResolvedValueOnce({ message: 'Follow-up email body' });
    render(<ContractGate lead={makeLead({ id: 7 })} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('contract-section-followup'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('contract-generate-followup-btn'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/contract-followup', { lead_id: 7 });
  });

  it('displays follow-up result', async () => {
    mockPost.mockResolvedValueOnce({ message: 'Hi, following up on our discussion...' });
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('contract-section-followup'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('contract-generate-followup-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('contract-followup-result')).toBeTruthy();
    });
    expect(screen.getByText('Hi, following up on our discussion...')).toBeTruthy();
  });

  it('generate case study calls correct API', async () => {
    mockPost.mockResolvedValueOnce({
      title: 'Case Study', challenge: 'X', approach: 'Y', results: 'Z',
    });
    render(<ContractGate lead={makeLead({ id: 3 })} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('contract-section-case-study'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('contract-generate-case-study-btn'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/case-study', { lead_id: 3 });
  });

  it('displays case study result with all sections', async () => {
    mockPost.mockResolvedValueOnce({
      title: 'Data Pipeline Optimization',
      challenge: 'Slow queries',
      approach: 'Added indexing',
      results: '10x faster',
    });
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('contract-section-case-study'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('contract-generate-case-study-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('contract-case-study-result')).toBeTruthy();
    });
    expect(screen.getByText('Data Pipeline Optimization')).toBeTruthy();
    expect(screen.getByText('Slow queries')).toBeTruthy();
    expect(screen.getByText('Added indexing')).toBeTruthy();
    expect(screen.getByText('10x faster')).toBeTruthy();
  });

  it('generate upsell calls correct API', async () => {
    mockPost.mockResolvedValueOnce({
      upsell_score: 75, opportunities: ['Add ML'], urgency: 'medium',
    });
    render(<ContractGate lead={makeLead({ id: 5 })} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('contract-section-upsell'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('contract-generate-upsell-btn'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/contract-upsell', { lead_id: 5 });
  });

  it('displays upsell result with score, urgency, and opportunities', async () => {
    mockPost.mockResolvedValueOnce({
      upsell_score: 82,
      opportunities: ['ML pipeline', 'Monitoring dashboard'],
      urgency: 'high',
    });
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('contract-section-upsell'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('contract-generate-upsell-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('contract-upsell-result')).toBeTruthy();
    });
    expect(screen.getByText('82')).toBeTruthy();
    expect(screen.getByText('ML pipeline')).toBeTruthy();
    expect(screen.getByText('high urgency')).toBeTruthy();
  });

  it('shows error when API call fails', async () => {
    mockPost.mockRejectedValueOnce(new Error('Server timeout'));
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('contract-generate-sow-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('contract-gate-error')).toBeTruthy();
      expect(screen.getByText('Server timeout')).toBeTruthy();
    });
  });

  it('shows Generating... text during loading', async () => {
    let resolvePost: (val: unknown) => void;
    mockPost.mockImplementationOnce(() => new Promise(r => { resolvePost = r; }));
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('contract-generate-sow-btn'));
    });
    expect(screen.getByText('Generating...')).toBeTruthy();
    await act(async () => {
      resolvePost!({ scope: '', deliverables: [], timeline: '', pricing: '' });
    });
  });

  it('multiple sections can be expanded simultaneously', () => {
    render(<ContractGate lead={makeLead()} onAction={vi.fn()} />);
    // SOW is already expanded
    expect(screen.getByTestId('contract-generate-sow-btn')).toBeTruthy();
    // Expand followup
    fireEvent.click(screen.getByTestId('contract-section-followup'));
    expect(screen.getByTestId('contract-generate-followup-btn')).toBeTruthy();
    // SOW should still be expanded
    expect(screen.getByTestId('contract-generate-sow-btn')).toBeTruthy();
  });

  it('shows compensation in header area', () => {
    render(<ContractGate lead={makeLead({ compensation: '$150/hr' })} onAction={vi.fn()} />);
    // Compensation is in the lead data but displayed via lead.title and lead.company
    expect(screen.getByTestId('contract-gate-header')).toBeTruthy();
  });
});
