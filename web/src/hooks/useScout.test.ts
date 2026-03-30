// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act, waitFor } from '@testing-library/react';
import { useScout } from './useScout';

const mockGet = vi.fn();
const mockPost = vi.fn();
const mockPatch = vi.fn();

vi.mock('../lib/api', () => ({
  api: {
    get: (...args: unknown[]) => mockGet(...args),
    post: (...args: unknown[]) => mockPost(...args),
    patch: (...args: unknown[]) => mockPatch(...args),
  },
}));

vi.mock('../lib/telemetry', () => ({
  reportError: vi.fn(),
  reportUsage: vi.fn(),
}));

function makeLead(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    title: 'Senior Engineer',
    company: 'Acme',
    type: 'job',
    source: 'linkedin',
    stage: 'screening',
    match_score: 85,
    compensation: '$150k',
    contact: 'recruiter@acme.com',
    location: 'Remote',
    notes: '',
    url: 'https://acme.com/job/1',
    created_at: '2026-03-30T10:00:00Z',
    updated_at: '2026-03-30T10:00:00Z',
    ...overrides,
  };
}

function makeAnalytics(overrides: Record<string, unknown> = {}) {
  return {
    by_type: { job: 5 },
    by_source: { linkedin: 3 },
    by_stage: { screening: 2 },
    conversion: [],
    weekly_trend: [],
    total_leads: 5,
    active_leads: 3,
    ...overrides,
  };
}

describe('useScout', () => {
  beforeEach(() => {
    mockGet.mockReset();
    mockPost.mockReset();
    mockPatch.mockReset();
    // Default: priority tab fetches leads on mount
    mockGet.mockResolvedValue([makeLead()]);
  });
  afterEach(() => cleanup());

  it('fetches leads on mount (priority tab)', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/scout/leads');
    expect(result.current.leads).toEqual([makeLead()]);
    expect(result.current.activeTab).toBe('priority');
  });

  it('starts in loading state', () => {
    mockGet.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useScout());
    expect(result.current.loading).toBe(true);
  });

  it('sets error on failure', async () => {
    mockGet.mockRejectedValue(new Error('Scout down'));
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('Scout down');
  });

  it('fetches leads on pipeline tab', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockClear();
    mockGet.mockResolvedValue([makeLead({ id: 2 })]);

    act(() => { result.current.setActiveTab('pipeline'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/scout/leads');
  });

  it('fetches analytics on analytics tab', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockResolvedValue(makeAnalytics());
    act(() => { result.current.setActiveTab('analytics'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/scout/analytics');
    expect(result.current.analytics).toEqual(makeAnalytics());
  });

  it('fetches sweep status and optimizations on actions tab', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const sweep = { running: false, platforms: [], started_at: '', progress: 0, results_found: 0 };
    const opts = [{ id: 1, type: 'resume', field: 'skills', current: 'old', suggested: 'new', reason: 'better', status: 'pending' }];
    mockGet.mockImplementation((path: string) => {
      if (path === '/api/scout/sweep/status') return Promise.resolve(sweep);
      if (path === '/api/scout/optimizations') return Promise.resolve(opts);
      return Promise.resolve(null);
    });

    act(() => { result.current.setActiveTab('actions'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.sweepStatus).toEqual(sweep);
    expect(result.current.optimizations).toEqual(opts);
  });

  it('fetches profile on profile tab', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const profile = { experience: [], projects: [], skills: ['Go'], education: [], certifications: [] };
    mockGet.mockResolvedValue(profile);

    act(() => { result.current.setActiveTab('profile'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockGet).toHaveBeenCalledWith('/api/scout/profile');
    expect(result.current.profile).toEqual(profile);
  });

  it('handles null leads response', async () => {
    mockGet.mockResolvedValue(null);
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.leads).toEqual([]);
  });

  it('addLead posts and refreshes', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue(makeLead({ id: 2 }));
    mockGet.mockResolvedValue([makeLead(), makeLead({ id: 2 })]);

    await act(async () => {
      await result.current.addLead({ title: 'New Lead' });
    });

    expect(mockPost).toHaveBeenCalledWith('/api/scout/leads', { title: 'New Lead' });
  });

  it('updateLead patches and refreshes', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPatch.mockResolvedValue(makeLead({ stage: 'qualified' }));
    mockGet.mockResolvedValue([makeLead({ stage: 'qualified' })]);

    await act(async () => {
      await result.current.updateLead(1, { stage: 'qualified' });
    });

    expect(mockPatch).toHaveBeenCalledWith('/api/scout/leads/1', { stage: 'qualified' });
  });

  it('triggerSweep posts platforms and refreshes actions', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue(undefined);
    mockGet.mockResolvedValue({ running: true, platforms: ['linkedin'], started_at: '', progress: 0, results_found: 0 });

    await act(async () => {
      await result.current.triggerSweep(['linkedin', 'indeed']);
    });

    expect(mockPost).toHaveBeenCalledWith('/api/scout/sweep', { platforms: ['linkedin', 'indeed'] });
  });

  it('syncPlatform posts and refreshes actions', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue(undefined);
    mockGet.mockResolvedValue(null);

    await act(async () => {
      await result.current.syncPlatform('github');
    });

    expect(mockPost).toHaveBeenCalledWith('/api/scout/sync', { platform: 'github' });
  });

  it('callAITool posts to AI endpoint', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue({ result: 'AI response' });

    let response: unknown;
    await act(async () => {
      response = await result.current.callAITool('match', { lead_id: 1 });
    });

    expect(mockPost).toHaveBeenCalledWith('/api/ai/match', { lead_id: 1 });
    expect(response).toEqual({ result: 'AI response' });
  });

  it('refresh re-fetches current tab', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockClear();
    mockGet.mockResolvedValue([]);

    act(() => { result.current.refresh(); });
    await waitFor(() => expect(mockGet).toHaveBeenCalledWith('/api/scout/leads'));
  });

  it('fetches intelligence tab data (scored leads)', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    const scored = [{ id: 1, title: 'Lead', company: 'Co', type: 'job', stage: 'screening', match_score: 90 }];
    mockGet.mockImplementation((path: string) => {
      if (path === '/api/scout/leads/scored') return Promise.resolve(scored);
      if (path === '/api/scout/sweep/status') return Promise.resolve(null);
      return Promise.resolve([]);
    });

    act(() => { result.current.setActiveTab('intelligence'); });
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.scoredLeads).toEqual(scored);
  });

  it('handles non-Error objects in catch', async () => {
    mockGet.mockRejectedValue('string error');
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('string error');
  });

  it('handles null optimizations response', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockGet.mockImplementation((path: string) => {
      if (path === '/api/scout/sweep/status') return Promise.resolve({ running: false, platforms: [], started_at: '', progress: 0, results_found: 0 });
      if (path === '/api/scout/optimizations') return Promise.resolve(null);
      return Promise.resolve(null);
    });

    act(() => { result.current.setActiveTab('actions'); });
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.optimizations).toEqual([]);
  });

  it('clears error on successful re-fetch', async () => {
    mockGet.mockRejectedValueOnce(new Error('Temporary failure'));
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('Temporary failure');

    mockGet.mockResolvedValue([makeLead()]);
    act(() => { result.current.refresh(); });
    await waitFor(() => expect(result.current.error).toBeNull());
  });

  it('addLead refreshes pipeline tab specifically', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue(makeLead({ id: 99 }));
    mockGet.mockClear();
    mockGet.mockResolvedValue([makeLead(), makeLead({ id: 99 })]);

    await act(async () => {
      await result.current.addLead({ title: 'Brand New' });
    });

    // Should have fetched /api/scout/leads (pipeline tab refresh)
    expect(mockGet).toHaveBeenCalledWith('/api/scout/leads');
  });

  it('updateLead refreshes current tab after patch', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPatch.mockResolvedValue(makeLead({ stage: 'interviewing' }));
    mockGet.mockClear();
    mockGet.mockResolvedValue([makeLead({ stage: 'interviewing' })]);

    await act(async () => {
      await result.current.updateLead(1, { stage: 'interviewing' });
    });

    // priority tab leads refresh
    expect(mockGet).toHaveBeenCalledWith('/api/scout/leads');
  });

  it('triggerSweep refreshes actions tab after post', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue(undefined);
    mockGet.mockClear();
    const sweep = { running: true, platforms: ['linkedin'], started_at: '', progress: 50, results_found: 3 };
    mockGet.mockImplementation((path: string) => {
      if (path === '/api/scout/sweep/status') return Promise.resolve(sweep);
      if (path === '/api/scout/optimizations') return Promise.resolve([]);
      return Promise.resolve(null);
    });

    await act(async () => {
      await result.current.triggerSweep(['linkedin']);
    });

    expect(mockGet).toHaveBeenCalledWith('/api/scout/sweep/status');
  });

  it('callAITool returns response data', async () => {
    const { result } = renderHook(() => useScout());
    await waitFor(() => expect(result.current.loading).toBe(false));

    mockPost.mockResolvedValue({ score: 85, breakdown: {} });

    let response: unknown;
    await act(async () => {
      response = await result.current.callAITool('freelance-score', { lead_id: 5 });
    });

    expect(mockPost).toHaveBeenCalledWith('/api/ai/freelance-score', { lead_id: 5 });
    expect(response).toEqual({ score: 85, breakdown: {} });
  });
});
