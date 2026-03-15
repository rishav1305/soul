import { useState, useEffect, useCallback } from 'react';
import { api } from '../lib/api';
import type { ModelInfo } from '../lib/types';

interface ModelsData {
  models: ModelInfo[];
  thinking_types: string[];
}

export function useModels() {
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [thinkingTypes, setThinkingTypes] = useState<string[]>(['disabled', 'adaptive', 'enabled']);
  const [loading, setLoading] = useState(true);

  const fetchModels = useCallback(async () => {
    try {
      const data = await api.get<ModelsData>('/api/models');
      if (data.models && data.models.length > 0) {
        setModels(data.models);
      }
      if (data.thinking_types) {
        setThinkingTypes(data.thinking_types);
      }
    } catch {
      // Keep defaults
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchModels();
  }, [fetchModels]);

  return { models, thinkingTypes, loading };
}
