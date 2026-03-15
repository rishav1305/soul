import { useState, useEffect, useCallback } from 'react';
import type { ObservePillar, ObserveOverview, ObserveTailResponse, ObserveTab, ObserveProduct } from '../lib/types';
import { api } from '../lib/api';
import { reportError, reportUsage } from '../lib/telemetry';

interface UseObserveReturn {
  pillars: ObservePillar[];
  overview: ObserveOverview | null;
  tail: ObserveTailResponse | null;
  loading: boolean;
  error: string | null;
  activeTab: ObserveTab;
  setActiveTab: (tab: ObserveTab) => void;
  product: ObserveProduct;
  setProduct: (product: ObserveProduct) => void;
  refresh: () => void;
}

export function useObserve(): UseObserveReturn {
  const [pillars, setPillars] = useState<ObservePillar[]>([]);
  const [overview, setOverview] = useState<ObserveOverview | null>(null);
  const [tail, setTail] = useState<ObserveTailResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<ObserveTab>('overview');
  const [product, setProduct] = useState<ObserveProduct>('');

  const fetchPillars = useCallback(async (prod: ObserveProduct) => {
    const query = prod ? `?product=${prod}` : '';
    const data = await api.get<ObservePillar[]>(`/api/observe/pillars${query}`);
    setPillars(data ?? []);
  }, []);

  const fetchTab = useCallback(async (tab: ObserveTab, prod: ObserveProduct) => {
    setLoading(true);
    setError(null);
    try {
      // Always fetch pillars for the health strip
      await fetchPillars(prod);

      switch (tab) {
        case 'overview': {
          const data = await api.get<ObserveOverview>('/api/observe/overview');
          setOverview(data);
          break;
        }
        case 'tail': {
          const data = await api.get<ObserveTailResponse>('/api/observe/tail?limit=100');
          setTail(data);
          break;
        }
        // Pillar tabs use pillars data — no extra fetch needed
        case 'performant':
        case 'robust':
        case 'resilient':
        case 'secure':
        case 'sovereign':
        case 'transparent':
          break;
      }
    } catch (err: unknown) {
      const message = err instanceof Error ? err.message : String(err);
      reportError('useObserve.fetch', err);
      setError(message);
    } finally {
      setLoading(false);
    }
  }, [fetchPillars]);

  useEffect(() => {
    fetchTab(activeTab, product);
  }, [activeTab, product, fetchTab]);

  const handleSetTab = useCallback((tab: ObserveTab) => {
    setActiveTab(tab);
    reportUsage('observe.tab', { tab });
  }, []);

  const handleSetProduct = useCallback((prod: ObserveProduct) => {
    setProduct(prod);
    reportUsage('observe.product', { product: prod });
  }, []);

  const refresh = useCallback(() => {
    fetchTab(activeTab, product);
  }, [activeTab, product, fetchTab]);

  return {
    pillars,
    overview,
    tail,
    loading,
    error,
    activeTab,
    setActiveTab: handleSetTab,
    product,
    setProduct: handleSetProduct,
    refresh,
  };
}
