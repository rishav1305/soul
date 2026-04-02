// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { NetworkingGate } from './NetworkingGate';

vi.mock('../../lib/api', () => ({
  api: {
    post: vi.fn().mockResolvedValue({ result: 'draft message' }),
  },
}));

function makeLead(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'CTO Intro',
    company: 'NetCo',
    type: 'job' as const,
    source: 'referral',
    stage: 'new',
    match_score: 75,
    compensation: '',
    contact: 'Jane Smith',
    location: 'NYC',
    notes: '',
    url: '',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

describe('NetworkingGate', () => {
  afterEach(() => cleanup());

  it('renders networking gate container', () => {
    render(<NetworkingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('networking-gate')).toBeTruthy();
  });

  it('shows contact name from lead.contact', () => {
    render(<NetworkingGate lead={makeLead({ contact: 'John Doe' })} onAction={vi.fn()} />);
    expect(screen.getByText('John Doe')).toBeTruthy();
  });

  it('falls back to title when contact empty', () => {
    render(<NetworkingGate lead={makeLead({ contact: '', title: 'VP Eng' })} onAction={vi.fn()} />);
    expect(screen.getByText('VP Eng')).toBeTruthy();
  });

  it('shows company', () => {
    render(<NetworkingGate lead={makeLead({ company: 'TechCo' })} onAction={vi.fn()} />);
    expect(screen.getByText('TechCo')).toBeTruthy();
  });

  it('shows Warm badge for score >= 70', () => {
    render(<NetworkingGate lead={makeLead({ match_score: 80 })} onAction={vi.fn()} />);
    expect(screen.getByTestId('networking-gate-warmth').textContent).toBe('Warm');
  });

  it('shows Lukewarm badge for score 40-69', () => {
    render(<NetworkingGate lead={makeLead({ match_score: 50 })} onAction={vi.fn()} />);
    expect(screen.getByTestId('networking-gate-warmth').textContent).toBe('Lukewarm');
  });

  it('shows Cold badge for score < 40', () => {
    render(<NetworkingGate lead={makeLead({ match_score: 20 })} onAction={vi.fn()} />);
    expect(screen.getByTestId('networking-gate-warmth').textContent).toBe('Cold');
  });

  it('renders channel tabs', () => {
    render(<NetworkingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('networking-gate-channels')).toBeTruthy();
    expect(screen.getByTestId('networking-gate-channel-linkedin')).toBeTruthy();
    expect(screen.getByTestId('networking-gate-channel-x')).toBeTruthy();
    expect(screen.getByTestId('networking-gate-channel-email')).toBeTruthy();
  });

  it('shows LinkedIn draft section by default', () => {
    render(<NetworkingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('networking-gate-section-linkedin')).toBeTruthy();
  });

  it('switches to X section on tab click', () => {
    render(<NetworkingGate lead={makeLead()} onAction={vi.fn()} />);
    fireEvent.click(screen.getByTestId('networking-gate-channel-x'));
    expect(screen.getByTestId('networking-gate-section-x')).toBeTruthy();
  });

  it('renders generate button', () => {
    render(<NetworkingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('networking-gate-generate-linkedin')).toBeTruthy();
  });

  it('renders footer action buttons', () => {
    render(<NetworkingGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('networking-gate-skip')).toBeTruthy();
    expect(screen.getByTestId('networking-gate-edit')).toBeTruthy();
    expect(screen.getByTestId('networking-gate-send')).toBeTruthy();
  });

  it('calls onAction with skip', () => {
    const onAction = vi.fn();
    render(<NetworkingGate lead={makeLead({ id: 8 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('networking-gate-skip'));
    expect(onAction).toHaveBeenCalledWith('skip', 8);
  });

  it('calls onAction with send', () => {
    const onAction = vi.fn();
    render(<NetworkingGate lead={makeLead({ id: 8 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('networking-gate-send'));
    expect(onAction).toHaveBeenCalledWith('send', 8);
  });
});
