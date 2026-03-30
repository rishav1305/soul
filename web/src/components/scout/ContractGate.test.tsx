// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ContractGate } from './ContractGate';

vi.mock('../../lib/api', () => ({
  api: {
    post: vi.fn().mockResolvedValue({
      scope: 'Build API',
      deliverables: ['API', 'Docs'],
      timeline: '4 weeks',
      pricing: '$10k',
    }),
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
  afterEach(() => cleanup());

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
});
