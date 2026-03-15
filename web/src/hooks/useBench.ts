import { useState, useEffect, useCallback } from 'react';
import { api } from '../lib/api';
import { reportError, reportUsage } from '../lib/telemetry';

type BenchTab = 'run' | 'results' | 'compare';

export interface BenchCategory {
  id: string;
  name: string;
  prompt_count: number;
}

export interface BenchResultSummary {
  id: string;
  model_name: string;
  timestamp: string;
  accuracy: number;
  latency_s: number;
  cars_ram: number;
  cars_size: number;
}

export interface CategoryScore {
  category: string;
  accuracy: number;
  prompts_total: number;
  prompts_correct: number;
}

export interface PromptDetail {
  id: string;
  category: string;
  prompt: string;
  expected: string;
  actual: string;
  correct: boolean;
  latency_ms: number;
}

export interface BenchResultDetail extends BenchResultSummary {
  tokens_per_sec: number;
  peak_ram_mb: number;
  category_scores: CategoryScore[];
  prompt_details: PromptDetail[];
}

export interface CompareData {
  result1: BenchResultDetail;
  result2: BenchResultDetail;
}

export interface RunConfig {
  endpoint: string;
  categories: string[];
  gpu: boolean;
  max_tokens: number;
}

export interface SmokeResult {
  name: string;
  passed: boolean;
  response_preview: string;
}

interface UseBenchReturn {
  categories: BenchCategory[];
  results: BenchResultSummary[];
  selectedResult: BenchResultDetail | null;
  compareData: CompareData | null;
  loading: boolean;
  error: string | null;
  activeTab: BenchTab;
  setActiveTab: (tab: BenchTab) => void;
  refresh: () => void;
  fetchResultDetail: (id: string) => Promise<void>;
  runBenchmark: (config: RunConfig) => Promise<void>;
  runSmoke: (endpoint: string) => Promise<SmokeResult[]>;
  compare: (id1: string, id2: string) => Promise<void>;
}

export function useBench(): UseBenchReturn {
  const [categories, setCategories] = useState<BenchCategory[]>([]);
  const [results, setResults] = useState<BenchResultSummary[]>([]);
  const [selectedResult, setSelectedResult] = useState<BenchResultDetail | null>(null);
  const [compareData, setCompareData] = useState<CompareData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<BenchTab>('run');

  const fetchData = useCallback(async (tab: BenchTab) => {
    setLoading(true);
    setError(null);
    try {
      switch (tab) {
        case 'run': {
          const data = await api.get<BenchCategory[]>('/api/bench/prompts');
          setCategories(data ?? []);
          break;
        }
        case 'results': {
          const data = await api.get<BenchResultSummary[]>('/api/bench/results');
          setResults(data ?? []);
          break;
        }
        case 'compare': {
          const data = await api.get<BenchResultSummary[]>('/api/bench/results');
          setResults(data ?? []);
          break;
        }
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useBench.fetch', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData(activeTab);
  }, [activeTab, fetchData]);

  const handleSetTab = useCallback((tab: BenchTab) => {
    setActiveTab(tab);
    reportUsage('bench.tab', { tab });
  }, []);

  const refresh = useCallback(() => {
    fetchData(activeTab);
  }, [activeTab, fetchData]);

  const fetchResultDetail = useCallback(async (id: string) => {
    try {
      const data = await api.get<BenchResultDetail>(`/api/bench/results/${id}`);
      setSelectedResult(data);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useBench.resultDetail', err);
      setError(message);
    }
  }, []);

  const runBenchmark = useCallback(async (config: RunConfig) => {
    setLoading(true);
    setError(null);
    try {
      await api.post('/api/bench/run', config);
      const data = await api.get<BenchResultSummary[]>('/api/bench/results');
      setResults(data ?? []);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useBench.runBenchmark', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  const runSmoke = useCallback(async (endpoint: string): Promise<SmokeResult[]> => {
    try {
      const data = await api.post<SmokeResult[]>('/api/bench/smoke', { endpoint });
      return data ?? [];
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useBench.runSmoke', err);
      setError(message);
      return [];
    }
  }, []);

  const compare = useCallback(async (id1: string, id2: string) => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.get<CompareData>(`/api/bench/compare?id1=${id1}&id2=${id2}`);
      setCompareData(data);
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useBench.compare', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, []);

  return {
    categories, results, selectedResult, compareData,
    loading, error, activeTab, setActiveTab: handleSetTab,
    refresh, fetchResultDetail, runBenchmark, runSmoke, compare,
  };
}
