import { useState, useEffect, useCallback } from 'react';
import { api } from '../lib/api';
import { reportError, reportUsage } from '../lib/telemetry';

// --- Types ---

export interface Challenge {
  id: string;
  category: string;
  difficulty: string;
  title: string;
  description: string;
  objective: string;
  points: number;
  max_turns: number;
  hints: string[];
  completed?: boolean;
}

export interface ChallengeSession {
  challenge_id: string;
  turn_count: number;
  response: string;
}

export interface Progress {
  total_points: number;
  completed: number;
  total_challenges: number;
  categories: Record<string, number>;
}

export interface AttackEntry {
  id: string;
  role: 'attacker' | 'defender';
  content: string;
  timestamp: string;
}

export interface SandboxConfig {
  name: string;
  systemPrompt: string;
  guardrails: string[];
  weaknessLevel: 'none' | 'low' | 'medium' | 'high';
}

export interface ScanResult {
  severity: 'critical' | 'high' | 'medium' | 'low';
  title: string;
  description: string;
  recommendation: string;
}

export interface FlagResult {
  correct: boolean;
  points_awarded: number;
  message: string;
}

// --- Hook ---

export type SentinelTab = 'challenges' | 'sandbox' | 'progress';

interface UseSentinelReturn {
  challenges: Challenge[];
  progress: Progress | null;
  activeChallenge: ChallengeSession | null;
  activeChallengeId: string | null;
  attackHistory: AttackEntry[];
  sandboxConfig: SandboxConfig;
  sandboxMessages: AttackEntry[];
  sandboxResponse: string | null;
  scanResults: ScanResult[];
  loading: boolean;
  error: string | null;
  activeTab: SentinelTab;
  setActiveTab: (tab: SentinelTab) => void;
  startChallenge: (id: string) => Promise<void>;
  submitFlag: (id: string, flag: string) => Promise<FlagResult | null>;
  attack: (payload: string, challengeId?: string) => Promise<void>;
  configureSandbox: (config: SandboxConfig) => Promise<void>;
  scanProduct: (product: string) => Promise<void>;
  sendSandboxMessage: (message: string) => Promise<void>;
  requestHint: (challengeId: string) => Promise<string | null>;
  exitChallenge: () => void;
  refresh: () => void;
}

const defaultSandboxConfig: SandboxConfig = {
  name: '',
  systemPrompt: '',
  guardrails: [],
  weaknessLevel: 'none',
};

// Helper to call Sentinel tool execution endpoints
async function toolExec<T>(tool: string, input: Record<string, unknown>): Promise<T> {
  return api.post<T>(`/api/sentinel/tools/${tool}/execute`, { input });
}

export function useSentinel(): UseSentinelReturn {
  const [challenges, setChallenges] = useState<Challenge[]>([]);
  const [progress, setProgress] = useState<Progress | null>(null);
  const [activeChallenge, setActiveChallenge] = useState<ChallengeSession | null>(null);
  const [activeChallengeId, setActiveChallengeId] = useState<string | null>(null);
  const [attackHistory, setAttackHistory] = useState<AttackEntry[]>([]);
  const [sandboxConfig, setSandboxConfig] = useState<SandboxConfig>(defaultSandboxConfig);
  const [sandboxMessages, setSandboxMessages] = useState<AttackEntry[]>([]);
  const [sandboxResponse, setSandboxResponse] = useState<string | null>(null);
  const [scanResults, setScanResults] = useState<ScanResult[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<SentinelTab>('challenges');

  const fetchData = useCallback(async (tab: SentinelTab) => {
    setLoading(true);
    setError(null);
    try {
      switch (tab) {
        case 'challenges': {
          const data = await toolExec<{ challenges: Challenge[] }>('challenge_list', {});
          setChallenges(data?.challenges ?? []);
          break;
        }
        case 'progress': {
          const data = await toolExec<Progress>('progress', {});
          setProgress(data);
          break;
        }
        case 'sandbox':
          // Sandbox is local-first; no initial fetch needed
          break;
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useSentinel.fetch', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData(activeTab);
  }, [activeTab, fetchData]);

  const handleSetTab = useCallback((tab: SentinelTab) => {
    setActiveTab(tab);
    reportUsage('sentinel.tab', { tab });
  }, []);

  const startChallenge = useCallback(async (id: string) => {
    setLoading(true);
    setError(null);
    try {
      const result = await toolExec<ChallengeSession>('challenge_start', { challenge_id: id });
      setActiveChallenge(result);
      setActiveChallengeId(id);
      setAttackHistory([]);
      reportUsage('sentinel.challenge.start', { challengeId: id });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useSentinel.startChallenge', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  const submitFlag = useCallback(async (id: string, flag: string): Promise<FlagResult | null> => {
    setError(null);
    try {
      const result = await toolExec<FlagResult>('challenge_submit', { challenge_id: id, flag });
      reportUsage('sentinel.challenge.submit', { challengeId: id, correct: result.correct });
      if (result.correct) {
        setActiveChallenge(null);
        setActiveChallengeId(null);
        setAttackHistory([]);
        // Refresh challenges list to update completion status
        const data = await toolExec<{ challenges: Challenge[] }>('challenge_list', {});
        setChallenges(data?.challenges ?? []);
      }
      return result;
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useSentinel.submitFlag', err);
      setError(message);
      return null;
    }
  }, []);

  const attack = useCallback(async (payload: string, challengeId?: string) => {
    setError(null);
    const userEntry: AttackEntry = {
      id: `atk-${Date.now()}`,
      role: 'attacker',
      content: payload,
      timestamp: new Date().toISOString(),
    };
    setAttackHistory(prev => [...prev, userEntry]);
    try {
      const result = await toolExec<ChallengeSession>('attack', {
        payload,
        ...(challengeId ? { challenge_id: challengeId } : {}),
      });
      const defenderEntry: AttackEntry = {
        id: `def-${Date.now()}`,
        role: 'defender',
        content: result.response,
        timestamp: new Date().toISOString(),
      };
      setAttackHistory(prev => [...prev, defenderEntry]);
      // Update turn count on active challenge
      if (activeChallenge) {
        setActiveChallenge({ ...activeChallenge, turn_count: result.turn_count });
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useSentinel.attack', err);
      setError(message);
    }
  }, [activeChallenge]);

  const configureSandbox = useCallback(async (config: SandboxConfig) => {
    setError(null);
    try {
      await toolExec('sandbox_config', {
        name: config.name,
        system_prompt: config.systemPrompt,
        guardrails: config.guardrails,
        weakness_level: config.weaknessLevel,
      });
      setSandboxConfig(config);
      setSandboxMessages([]);
      setSandboxResponse(null);
      reportUsage('sentinel.sandbox.config', { name: config.name });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useSentinel.configureSandbox', err);
      setError(message);
    }
  }, []);

  const sendSandboxMessage = useCallback(async (message: string) => {
    setError(null);
    const userEntry: AttackEntry = {
      id: `sbx-user-${Date.now()}`,
      role: 'attacker',
      content: message,
      timestamp: new Date().toISOString(),
    };
    setSandboxMessages(prev => [...prev, userEntry]);
    try {
      const result = await toolExec<{ response: string }>('defend', { prompt: message });
      setSandboxResponse(result.response);
      const defenderEntry: AttackEntry = {
        id: `sbx-def-${Date.now()}`,
        role: 'defender',
        content: result.response,
        timestamp: new Date().toISOString(),
      };
      setSandboxMessages(prev => [...prev, defenderEntry]);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      reportError('useSentinel.sandboxMessage', err);
      setError(msg);
    }
  }, []);

  const scanProduct = useCallback(async (product: string) => {
    setLoading(true);
    setError(null);
    try {
      const results = await toolExec<{ findings: ScanResult[] }>('scan', { product });
      setScanResults(results?.findings ?? []);
      reportUsage('sentinel.scan', { product });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useSentinel.scan', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  const requestHint = useCallback(async (challengeId: string): Promise<string | null> => {
    try {
      const result = await toolExec<{ hint: string }>('challenge_submit', { challenge_id: challengeId, request_hint: true });
      return result.hint;
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useSentinel.hint', err);
      setError(message);
      return null;
    }
  }, []);

  const exitChallenge = useCallback(() => {
    setActiveChallenge(null);
    setActiveChallengeId(null);
    setAttackHistory([]);
  }, []);

  const refresh = useCallback(() => {
    fetchData(activeTab);
  }, [activeTab, fetchData]);

  return {
    challenges,
    progress,
    activeChallenge,
    activeChallengeId,
    attackHistory,
    sandboxConfig,
    sandboxMessages,
    sandboxResponse,
    scanResults,
    loading,
    error,
    activeTab,
    setActiveTab: handleSetTab,
    startChallenge,
    submitFlag,
    attack,
    configureSandbox,
    scanProduct,
    sendSandboxMessage,
    requestHint,
    exitChallenge,
    refresh,
  };
}
