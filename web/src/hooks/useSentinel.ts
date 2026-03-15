import { useState, useEffect, useCallback } from 'react';
import { api } from '../lib/api';
import { reportError, reportUsage } from '../lib/telemetry';

// --- Types ---

export interface SentinelChallenge {
  id: number;
  title: string;
  category: string;
  difficulty: 'easy' | 'medium' | 'hard' | 'expert';
  points: number;
  description: string;
  objective: string;
  maxTurns: number;
  hintCount: number;
  completed: boolean;
  completedAt?: string;
}

export interface SentinelProgress {
  totalPoints: number;
  challengesCompleted: number;
  challengesTotal: number;
  categoryBreakdown: CategoryProgress[];
}

export interface CategoryProgress {
  category: string;
  completed: number;
  total: number;
  points: number;
  maxPoints: number;
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

export interface ChallengeStartResult {
  sessionId: string;
  description: string;
  objective: string;
  maxTurns: number;
  hints: number;
}

export interface AttackResult {
  response: string;
  turnNumber: number;
  turnsRemaining: number;
  flagCaptured: boolean;
}

export interface FlagResult {
  correct: boolean;
  pointsAwarded: number;
  message: string;
}

// --- Hook ---

export type SentinelTab = 'challenges' | 'sandbox' | 'progress';

interface UseSentinelReturn {
  challenges: SentinelChallenge[];
  progress: SentinelProgress | null;
  activeChallenge: ChallengeStartResult | null;
  activeChallengeId: number | null;
  attackHistory: AttackEntry[];
  sandboxConfig: SandboxConfig;
  sandboxMessages: AttackEntry[];
  scanResults: ScanResult[];
  loading: boolean;
  error: string | null;
  activeTab: SentinelTab;
  setActiveTab: (tab: SentinelTab) => void;
  startChallenge: (id: number) => Promise<void>;
  submitFlag: (id: number, flag: string) => Promise<FlagResult | null>;
  attack: (mode: string, payload: string, challengeId?: number) => Promise<void>;
  configureSandbox: (config: SandboxConfig) => Promise<void>;
  scanProduct: (product: string) => Promise<void>;
  sendSandboxMessage: (message: string) => Promise<void>;
  requestHint: (challengeId: number) => Promise<string | null>;
  exitChallenge: () => void;
  refresh: () => void;
}

const defaultSandboxConfig: SandboxConfig = {
  name: '',
  systemPrompt: '',
  guardrails: [],
  weaknessLevel: 'none',
};

export function useSentinel(): UseSentinelReturn {
  const [challenges, setChallenges] = useState<SentinelChallenge[]>([]);
  const [progress, setProgress] = useState<SentinelProgress | null>(null);
  const [activeChallenge, setActiveChallenge] = useState<ChallengeStartResult | null>(null);
  const [activeChallengeId, setActiveChallengeId] = useState<number | null>(null);
  const [attackHistory, setAttackHistory] = useState<AttackEntry[]>([]);
  const [sandboxConfig, setSandboxConfig] = useState<SandboxConfig>(defaultSandboxConfig);
  const [sandboxMessages, setSandboxMessages] = useState<AttackEntry[]>([]);
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
          const data = await api.get<SentinelChallenge[]>('/api/sentinel/challenges');
          setChallenges(data ?? []);
          break;
        }
        case 'progress': {
          const data = await api.get<SentinelProgress>('/api/sentinel/progress');
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

  const startChallenge = useCallback(async (id: number) => {
    setLoading(true);
    setError(null);
    try {
      const result = await api.post<ChallengeStartResult>('/api/sentinel/challenges/start', { challengeId: id });
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

  const submitFlag = useCallback(async (id: number, flag: string): Promise<FlagResult | null> => {
    setError(null);
    try {
      const result = await api.post<FlagResult>('/api/sentinel/challenges/submit', { challengeId: id, flag });
      reportUsage('sentinel.challenge.submit', { challengeId: id, correct: result.correct });
      if (result.correct) {
        setActiveChallenge(null);
        setActiveChallengeId(null);
        setAttackHistory([]);
        // Refresh challenges list to update completion status
        const data = await api.get<SentinelChallenge[]>('/api/sentinel/challenges');
        setChallenges(data ?? []);
      }
      return result;
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useSentinel.submitFlag', err);
      setError(message);
      return null;
    }
  }, []);

  const attack = useCallback(async (mode: string, payload: string, challengeId?: number) => {
    setError(null);
    const userEntry: AttackEntry = {
      id: `atk-${Date.now()}`,
      role: 'attacker',
      content: payload,
      timestamp: new Date().toISOString(),
    };
    setAttackHistory(prev => [...prev, userEntry]);
    try {
      const result = await api.post<AttackResult>('/api/sentinel/attack', { mode, payload, challengeId });
      const defenderEntry: AttackEntry = {
        id: `def-${Date.now()}`,
        role: 'defender',
        content: result.response,
        timestamp: new Date().toISOString(),
      };
      setAttackHistory(prev => [...prev, defenderEntry]);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useSentinel.attack', err);
      setError(message);
    }
  }, []);

  const configureSandbox = useCallback(async (config: SandboxConfig) => {
    setError(null);
    try {
      await api.post('/api/sentinel/sandbox/config', config);
      setSandboxConfig(config);
      setSandboxMessages([]);
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
      const result = await api.post<{ response: string }>('/api/sentinel/attack', { mode: 'sandbox', payload: message });
      const defenderEntry: AttackEntry = {
        id: `sbx-def-${Date.now()}`,
        role: 'defender',
        content: result.response,
        timestamp: new Date().toISOString(),
      };
      setSandboxMessages(prev => [...prev, defenderEntry]);
    } catch (err: unknown) {
      const message_str = err instanceof Error ? err.message : String(err);
      reportError('useSentinel.sandboxMessage', err);
      setError(message_str);
    }
  }, []);

  const scanProduct = useCallback(async (product: string) => {
    setLoading(true);
    setError(null);
    try {
      const results = await api.post<ScanResult[]>('/api/sentinel/scan', { product });
      setScanResults(results ?? []);
      reportUsage('sentinel.scan', { product });
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useSentinel.scan', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  const requestHint = useCallback(async (challengeId: number): Promise<string | null> => {
    try {
      const result = await api.post<{ hint: string }>('/api/sentinel/challenges/hint', { challengeId });
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
