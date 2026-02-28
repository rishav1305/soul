import { useState, useCallback, useMemo } from 'react';
import type { LayoutState, PanelState, TaskView, GridSubView, TaskFilters } from '../lib/types.ts';

const STORAGE_KEY = 'soul-layout';

const DEFAULT_STATE: LayoutState = {
  soulState: 'rail',
  chatState: 'open',
  taskState: 'open',
  taskView: 'kanban',
  gridSubView: 'grid',
  panelWidth: null,
  filters: { stage: 'all', priority: 'all', product: 'all' },
};

function loadState(): LayoutState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_STATE;
    const parsed = JSON.parse(raw) as Partial<LayoutState>;
    return { ...DEFAULT_STATE, ...parsed, filters: { ...DEFAULT_STATE.filters, ...parsed.filters } };
  } catch {
    return DEFAULT_STATE;
  }
}

function saveState(state: LayoutState): void {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(state));
  } catch {
    // localStorage full or blocked — silently ignore
  }
}

/** Returns auto-computed task panel width % based on task count. */
export function autoWidth(taskCount: number): number {
  if (taskCount === 0) return 15;
  if (taskCount <= 3) return 25;
  if (taskCount <= 10) return 40;
  if (taskCount <= 20) return 55;
  return 75;
}

export function useLayoutStore() {
  const [state, _setState] = useState<LayoutState>(loadState);

  const setState = useCallback((updater: (prev: LayoutState) => LayoutState) => {
    _setState((prev) => {
      const next = updater(prev);
      saveState(next);
      return next;
    });
  }, []);

  const canCollapse = useCallback(
    (panel: 'chat' | 'task'): boolean => {
      if (panel === 'chat') return state.taskState !== 'rail';
      return state.chatState !== 'rail';
    },
    [state.chatState, state.taskState],
  );

  const setSoulState = useCallback(
    (s: PanelState) => {
      setState((prev) => ({ ...prev, soulState: s }));
    },
    [setState],
  );

  const setChatState = useCallback(
    (s: PanelState) => {
      setState((prev) => {
        if (s === 'rail' && prev.taskState === 'rail') return prev; // block both-rail
        return { ...prev, chatState: s };
      });
    },
    [setState],
  );

  const setTaskState = useCallback(
    (s: PanelState) => {
      setState((prev) => {
        if (s === 'rail' && prev.chatState === 'rail') return prev; // block both-rail
        return { ...prev, taskState: s };
      });
    },
    [setState],
  );

  const setTaskView = useCallback(
    (v: TaskView) => {
      setState((prev) => ({ ...prev, taskView: v, taskState: 'open' }));
    },
    [setState],
  );

  const setGridSubView = useCallback(
    (v: GridSubView) => {
      setState((prev) => ({ ...prev, gridSubView: v }));
    },
    [setState],
  );

  const setPanelWidth = useCallback(
    (w: number | null) => {
      setState((prev) => ({ ...prev, panelWidth: w }));
    },
    [setState],
  );

  const setFilters = useCallback(
    (partial: Partial<TaskFilters>) => {
      setState((prev) => ({ ...prev, filters: { ...prev.filters, ...partial } }));
    },
    [setState],
  );

  return useMemo(
    () => ({
      ...state,
      setSoulState,
      setChatState,
      setTaskState,
      setTaskView,
      setGridSubView,
      setPanelWidth,
      setFilters,
      canCollapse,
    }),
    [state, setSoulState, setChatState, setTaskState, setTaskView, setGridSubView, setPanelWidth, setFilters, canCollapse],
  );
}
