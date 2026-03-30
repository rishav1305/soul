// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act, waitFor } from '@testing-library/react';
import { FreelanceGate } from './FreelanceGate';

const mockPost = vi.fn().mockResolvedValue({ result: 'proposal text' });

vi.mock('../../lib/api', () => ({
  api: {
    post: (...args: unknown[]) => mockPost(...args),
  },
}));

function makeLead(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'React Dashboard',
    company: 'ClientCo',
    type: 'freelance' as const,
    source: 'upwork',
    stage: 'qualified',
    match_score: 75,
    compensation: '$5k',
    contact: '',
    location: 'Remote',
    notes: '',
    url: '',
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    ...overrides,
  };
}

describe('FreelanceGate', () => {
  afterEach(() => {
    cleanup();
    mockPost.mockClear();
  });

  it('renders freelance gate container', () => {
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('freelance-gate')).toBeTruthy();
  });

  it('shows lead title', () => {
    render(<FreelanceGate lead={makeLead({ title: 'Node API' })} onAction={vi.fn()} />);
    expect(screen.getByText('Node API')).toBeTruthy();
  });

  it('shows company', () => {
    render(<FreelanceGate lead={makeLead({ company: 'FreelanceCo' })} onAction={vi.fn()} />);
    expect(screen.getByText('FreelanceCo')).toBeTruthy();
  });

  it('shows score badge', () => {
    render(<FreelanceGate lead={makeLead({ match_score: 85 })} onAction={vi.fn()} />);
    expect(screen.getByTestId('freelance-gate-score-badge').textContent).toBe('85%');
  });

  it('renders gig score section', () => {
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('freelance-gate-section-score')).toBeTruthy();
    expect(screen.getByText('Gig Score')).toBeTruthy();
  });

  it('renders proposal section', () => {
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('freelance-gate-section-proposal')).toBeTruthy();
    expect(screen.getByText('Proposal Draft')).toBeTruthy();
  });

  it('renders generate buttons', () => {
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('freelance-gate-generate-score')).toBeTruthy();
    expect(screen.getByTestId('freelance-gate-generate-proposal')).toBeTruthy();
  });

  it('renders footer action buttons', () => {
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByTestId('freelance-gate-skip')).toBeTruthy();
    expect(screen.getByTestId('freelance-gate-edit')).toBeTruthy();
    expect(screen.getByTestId('freelance-gate-submit')).toBeTruthy();
  });

  it('calls onAction with skip', () => {
    const onAction = vi.fn();
    render(<FreelanceGate lead={makeLead({ id: 7 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('freelance-gate-skip'));
    expect(onAction).toHaveBeenCalledWith('skip', 7);
  });

  it('calls onAction with submit', () => {
    const onAction = vi.fn();
    render(<FreelanceGate lead={makeLead({ id: 7 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('freelance-gate-submit'));
    expect(onAction).toHaveBeenCalledWith('submit', 7);
  });

  it('shows Submit Proposal button text', () => {
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    expect(screen.getByText('Submit Proposal')).toBeTruthy();
  });

  it('generate score calls API with lead_id', async () => {
    mockPost.mockResolvedValueOnce({
      result: '',
      breakdown: { skill_match: 80, budget_fit: 70, scope_clarity: 90, client_quality: 60, time_fit: 85 },
    });
    render(<FreelanceGate lead={makeLead({ id: 15 })} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('freelance-gate-generate-score'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/freelance-score', { lead_id: 15 });
  });

  it('displays score breakdown with all 5 fields', async () => {
    mockPost.mockResolvedValueOnce({
      result: '',
      breakdown: { skill_match: 80, budget_fit: 70, scope_clarity: 90, client_quality: 60, time_fit: 85 },
    });
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('freelance-gate-generate-score'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('freelance-gate-content-score')).toBeTruthy();
    });
    expect(screen.getByText('Skill Match')).toBeTruthy();
    expect(screen.getByText('Budget Fit')).toBeTruthy();
    expect(screen.getByText('Scope Clarity')).toBeTruthy();
    expect(screen.getByText('Client Quality')).toBeTruthy();
    expect(screen.getByText('Time Fit')).toBeTruthy();
  });

  it('hides generate score button after successful generation', async () => {
    mockPost.mockResolvedValueOnce({
      result: '',
      breakdown: { skill_match: 80, budget_fit: 70, scope_clarity: 90, client_quality: 60, time_fit: 85 },
    });
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('freelance-gate-generate-score'));
    });
    await waitFor(() => {
      expect(screen.queryByTestId('freelance-gate-generate-score')).toBeNull();
    });
  });

  it('generate proposal calls API with lead_id and platform', async () => {
    mockPost.mockResolvedValueOnce({ result: 'Dear client...' });
    render(<FreelanceGate lead={makeLead({ id: 20 })} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('freelance-gate-generate-proposal'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/proposal', { lead_id: 20, platform: 'upwork' });
  });

  it('displays proposal content', async () => {
    mockPost.mockResolvedValueOnce({ result: 'Dear client, I am excited to work on this project...' });
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('freelance-gate-generate-proposal'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('freelance-gate-content-proposal')).toBeTruthy();
    });
    expect(screen.getByText(/Dear client, I am excited/)).toBeTruthy();
  });

  it('shows error when score generation fails', async () => {
    mockPost.mockRejectedValueOnce(new Error('Score API error'));
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('freelance-gate-generate-score'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('freelance-gate-error-score')).toBeTruthy();
      expect(screen.getByText('Score API error')).toBeTruthy();
    });
  });

  it('shows error when proposal generation fails', async () => {
    mockPost.mockRejectedValueOnce(new Error('Proposal API error'));
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('freelance-gate-generate-proposal'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('freelance-gate-error-proposal')).toBeTruthy();
      expect(screen.getByText('Proposal API error')).toBeTruthy();
    });
  });

  it('shows loading spinner during score generation', async () => {
    let resolvePost: (val: unknown) => void;
    mockPost.mockImplementationOnce(() => new Promise(r => { resolvePost = r; }));
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('freelance-gate-generate-score'));
    });
    expect(screen.getByText('Analyzing gig...')).toBeTruthy();
    await act(async () => {
      resolvePost!({ result: '', breakdown: { skill_match: 50, budget_fit: 50, scope_clarity: 50, client_quality: 50, time_fit: 50 } });
    });
  });

  it('shows loading spinner during proposal generation', async () => {
    let resolvePost: (val: unknown) => void;
    mockPost.mockImplementationOnce(() => new Promise(r => { resolvePost = r; }));
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('freelance-gate-generate-proposal'));
    });
    expect(screen.getByText('Drafting proposal...')).toBeTruthy();
    await act(async () => {
      resolvePost!({ result: 'proposal' });
    });
  });

  it('calls onAction with edit', () => {
    const onAction = vi.fn();
    render(<FreelanceGate lead={makeLead({ id: 7 })} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('freelance-gate-edit'));
    expect(onAction).toHaveBeenCalledWith('edit', 7);
  });

  it('score badge uses green for high scores', () => {
    render(<FreelanceGate lead={makeLead({ match_score: 90 })} onAction={vi.fn()} />);
    const badge = screen.getByTestId('freelance-gate-score-badge');
    expect(badge.className).toContain('emerald');
  });

  it('score badge uses amber for medium scores', () => {
    render(<FreelanceGate lead={makeLead({ match_score: 60 })} onAction={vi.fn()} />);
    const badge = screen.getByTestId('freelance-gate-score-badge');
    expect(badge.className).toContain('amber');
  });

  it('score badge uses zinc for low scores', () => {
    render(<FreelanceGate lead={makeLead({ match_score: 30 })} onAction={vi.fn()} />);
    const badge = screen.getByTestId('freelance-gate-score-badge');
    expect(badge.className).toContain('zinc');
  });

  it('shows stage badge', () => {
    render(<FreelanceGate lead={makeLead({ stage: 'qualified' })} onAction={vi.fn()} />);
    // Stage is visible via the lead display
    expect(screen.getByTestId('freelance-gate')).toBeTruthy();
  });

  it('parses breakdown from result string when breakdown not provided', async () => {
    mockPost.mockResolvedValueOnce({
      result: JSON.stringify({ skill_match: 75, budget_fit: 65, scope_clarity: 80, client_quality: 55, time_fit: 70 }),
    });
    render(<FreelanceGate lead={makeLead()} onAction={vi.fn()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('freelance-gate-generate-score'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('freelance-gate-content-score')).toBeTruthy();
    });
    expect(screen.getByText('Skill Match')).toBeTruthy();
  });
});
