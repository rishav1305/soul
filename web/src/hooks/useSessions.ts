import { useState, useEffect, useCallback } from 'react';
import type { ChatSession } from '../lib/types.ts';

export function useSessions() {
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<number | null>(null);

  const fetchSessions = useCallback(async () => {
    try {
      const res = await fetch('/api/sessions');
      if (!res.ok) return;
      const data: ChatSession[] = await res.json();
      setSessions(data);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => { fetchSessions(); }, [fetchSessions]);

  const createSession = useCallback(async () => {
    const res = await fetch('/api/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: '' }),
    });
    if (!res.ok) throw new Error('Failed to create session');
    const session: ChatSession = await res.json();
    setSessions(prev => [session, ...prev].slice(0, 10));
    setActiveSessionId(session.id);
    return session;
  }, []);

  const switchSession = useCallback((id: number) => {
    setActiveSessionId(id);
  }, []);

  return { sessions, activeSessionId, createSession, switchSession, fetchSessions };
}
