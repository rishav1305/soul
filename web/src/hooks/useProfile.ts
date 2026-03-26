import { useState, useEffect, useCallback } from 'react';
import { authFetch } from '../lib/api.ts';
import type { ProfileData } from '../lib/types.ts';

async function callProfileTool(tool: string, input: Record<string, unknown> = {}) {
  const resp = await authFetch(`/api/tools/scout__${tool}/execute`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ input }),
  });
  return resp.json();
}

export function useProfile() {
  const [profile, setProfile] = useState<ProfileData | null>(null);
  const [loading, setLoading] = useState(true);
  const [pulling, setPulling] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const fetchProfile = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await callProfileTool('profile');
      if (res?.structured_json) {
        setProfile(JSON.parse(res.structured_json));
      } else if (res?.output && !res.success) {
        setError(res.output);
      }
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load profile');
    } finally {
      setLoading(false);
    }
  }, []);

  const pullFromSupabase = useCallback(async () => {
    setPulling(true);
    setError(null);
    try {
      const res = await callProfileTool('profile_pull');
      if (!res?.success) {
        setError(res?.output ?? 'Pull failed');
        return;
      }
      await fetchProfile();
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Pull failed');
    } finally {
      setPulling(false);
    }
  }, [fetchProfile]);

  useEffect(() => {
    fetchProfile();
  }, [fetchProfile]);

  return { profile, loading, pulling, error, fetchProfile, pullFromSupabase };
}
