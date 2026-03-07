import { useState, useEffect, useCallback } from 'react';
import { useWebSocket } from './useWebSocket.ts';
import type { ChatSession, WSMessage } from '../lib/types.ts';

const SESSION_KEY = 'soul-active-session';

function loadSessionId(): number | null {
  try {
    const raw = localStorage.getItem(SESSION_KEY);
    if (!raw) return null;
    const id = Number(raw);
    return Number.isFinite(id) ? id : null;
  } catch { return null; }
}

function saveSessionId(id: number | null): void {
  try {
    if (id === null) localStorage.removeItem(SESSION_KEY);
    else localStorage.setItem(SESSION_KEY, String(id));
  } catch { /* ignore */ }
}

export function useSessions() {
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [activeSessionId, _setActiveSessionId] = useState<number | null>(loadSessionId);
  const { onMessage } = useWebSocket();

  const setActiveSessionId = useCallback((id: number | null) => {
    _setActiveSessionId(id);
    saveSessionId(id);
  }, []);

  const fetchSessions = useCallback(async () => {
    try {
      const res = await fetch('/api/sessions');
      if (!res.ok) return;
      const data: ChatSession[] = await res.json();
      setSessions(data);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => { fetchSessions(); }, [fetchSessions]);

  // Listen for session.updated from the server (triggered by summary generation).
  useEffect(() => {
    const unsub = onMessage((msg: WSMessage) => {
      if (msg.type !== 'session.updated') return;
      const data = msg.data as { session_id: number; title: string; summary: string; model: string };
      if (!data?.session_id) return;
      setSessions(prev => prev.map(s =>
        s.id === data.session_id
          ? { ...s, title: data.title || s.title, summary: data.summary || s.summary, model: data.model || s.model }
          : s,
      ));
    });
    return unsub;
  }, [onMessage]);

  const createSession = useCallback(async () => {
    const res = await fetch('/api/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ title: '' }),
    });
    if (!res.ok) throw new Error('Failed to create session');
    const session: ChatSession = await res.json();
    setSessions(prev => [session, ...prev].slice(0, 30));
    setActiveSessionId(session.id);
    return session;
  }, []);

  const switchSession = useCallback((id: number) => {
    setActiveSessionId(id);
  }, []);

  return { sessions, activeSessionId, createSession, switchSession, fetchSessions };
}
