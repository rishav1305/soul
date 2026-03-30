// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, waitFor, cleanup, act } from '@testing-library/react';
import { useProfile } from './useProfile';

// Mock authFetch
const mockAuthFetch = vi.fn();
vi.mock('../lib/api.ts', () => ({
  authFetch: (...args: unknown[]) => mockAuthFetch(...args),
}));

describe('useProfile', () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });
  afterEach(() => cleanup());

  it('starts in loading state', () => {
    mockAuthFetch.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useProfile());
    expect(result.current.loading).toBe(true);
    expect(result.current.profile).toBeNull();
    expect(result.current.error).toBeNull();
  });

  it('fetches profile via scout tool endpoint', async () => {
    mockAuthFetch.mockResolvedValue({
      json: () => Promise.resolve({
        structured_json: JSON.stringify({ name: 'Rishav', title: 'Engineer' }),
      }),
    });

    const { result } = renderHook(() => useProfile());

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(mockAuthFetch).toHaveBeenCalledWith(
      '/api/tools/scout__profile/execute',
      expect.objectContaining({ method: 'POST' }),
    );
    expect(result.current.profile).toEqual({ name: 'Rishav', title: 'Engineer' });
    expect(result.current.error).toBeNull();
  });

  it('sets error when API returns non-success output', async () => {
    mockAuthFetch.mockResolvedValue({
      json: () => Promise.resolve({
        success: false,
        output: 'Profile not found',
      }),
    });

    const { result } = renderHook(() => useProfile());

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.profile).toBeNull();
    expect(result.current.error).toBe('Profile not found');
  });

  it('sets error on fetch failure', async () => {
    mockAuthFetch.mockRejectedValue(new Error('Network error'));

    const { result } = renderHook(() => useProfile());

    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.profile).toBeNull();
    expect(result.current.error).toBe('Network error');
  });

  it('pullFromSupabase calls profile_pull then refreshes', async () => {
    let callCount = 0;
    mockAuthFetch.mockImplementation((url: string) => {
      callCount++;
      if (url.includes('profile_pull')) {
        return Promise.resolve({
          json: () => Promise.resolve({ success: true }),
        });
      }
      return Promise.resolve({
        json: () => Promise.resolve({
          structured_json: JSON.stringify({ name: 'Updated' }),
        }),
      });
    });

    const { result } = renderHook(() => useProfile());

    await waitFor(() => expect(result.current.loading).toBe(false));

    await result.current.pullFromSupabase();

    expect(result.current.pulling).toBe(false);
    // Should have called profile_pull
    const pullCalls = mockAuthFetch.mock.calls.filter(
      (call: unknown[]) => (call[0] as string).includes('profile_pull'),
    );
    expect(pullCalls.length).toBe(1);
  });

  it('sets error when pull fails', async () => {
    mockAuthFetch.mockImplementation((url: string) => {
      if (url.includes('profile_pull')) {
        return Promise.resolve({
          json: () => Promise.resolve({ success: false, output: 'DB connection failed' }),
        });
      }
      return Promise.resolve({
        json: () => Promise.resolve({ structured_json: '{}' }),
      });
    });

    const { result } = renderHook(() => useProfile());

    await waitFor(() => expect(result.current.loading).toBe(false));

    await result.current.pullFromSupabase();

    await waitFor(() => {
      expect(result.current.error).toBe('DB connection failed');
    });
  });

  it('pullFromSupabase sets pulling true during pull', async () => {
    let resolvePull: ((v: unknown) => void) | null = null;
    mockAuthFetch.mockImplementation((url: string) => {
      if (url.includes('profile_pull')) {
        return new Promise((resolve) => { resolvePull = resolve; });
      }
      return Promise.resolve({
        json: () => Promise.resolve({ structured_json: '{}' }),
      });
    });

    const { result } = renderHook(() => useProfile());
    await waitFor(() => expect(result.current.loading).toBe(false));

    let pullPromise: Promise<void>;
    act(() => {
      pullPromise = result.current.pullFromSupabase();
    });

    // pulling should be true while in-flight
    await waitFor(() => expect(result.current.pulling).toBe(true));

    // Resolve it
    await act(async () => {
      resolvePull!({ json: () => Promise.resolve({ success: true }) });
      await pullPromise!;
    });
    expect(result.current.pulling).toBe(false);
  });

  it('pullFromSupabase handles network error', async () => {
    mockAuthFetch.mockImplementation((url: string) => {
      if (url.includes('profile_pull')) {
        return Promise.reject(new Error('Network timeout'));
      }
      return Promise.resolve({
        json: () => Promise.resolve({ structured_json: '{}' }),
      });
    });

    const { result } = renderHook(() => useProfile());
    await waitFor(() => expect(result.current.loading).toBe(false));

    await result.current.pullFromSupabase();

    await waitFor(() => {
      expect(result.current.error).toBe('Network timeout');
    });
    expect(result.current.pulling).toBe(false);
  });

  it('handles non-Error in fetch catch', async () => {
    mockAuthFetch.mockRejectedValue('string fetch error');

    const { result } = renderHook(() => useProfile());
    await waitFor(() => expect(result.current.loading).toBe(false));

    expect(result.current.error).toBe('Failed to load profile');
  });

  it('handles non-Error in pull catch', async () => {
    mockAuthFetch.mockImplementation((url: string) => {
      if (url.includes('profile_pull')) {
        return Promise.reject('pull string error');
      }
      return Promise.resolve({
        json: () => Promise.resolve({ structured_json: '{}' }),
      });
    });

    const { result } = renderHook(() => useProfile());
    await waitFor(() => expect(result.current.loading).toBe(false));

    await result.current.pullFromSupabase();

    await waitFor(() => {
      expect(result.current.error).toBe('Pull failed');
    });
  });

  it('fetchProfile clears error on success', async () => {
    // First call: error
    mockAuthFetch.mockRejectedValueOnce(new Error('Temporary error'));

    const { result } = renderHook(() => useProfile());
    await waitFor(() => expect(result.current.loading).toBe(false));
    expect(result.current.error).toBe('Temporary error');

    // Second call: success
    mockAuthFetch.mockResolvedValueOnce({
      json: () => Promise.resolve({
        structured_json: JSON.stringify({ name: 'Test' }),
      }),
    });

    await result.current.fetchProfile();
    await waitFor(() => expect(result.current.error).toBeNull());
    expect(result.current.profile).toEqual({ name: 'Test' });
  });

  it('pull failure with no output defaults error message', async () => {
    mockAuthFetch.mockImplementation((url: string) => {
      if (url.includes('profile_pull')) {
        return Promise.resolve({
          json: () => Promise.resolve({ success: false }),
        });
      }
      return Promise.resolve({
        json: () => Promise.resolve({ structured_json: '{}' }),
      });
    });

    const { result } = renderHook(() => useProfile());
    await waitFor(() => expect(result.current.loading).toBe(false));

    await result.current.pullFromSupabase();

    await waitFor(() => {
      expect(result.current.error).toBe('Pull failed');
    });
  });
});
