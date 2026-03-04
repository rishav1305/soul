import { useState, useCallback, useMemo } from 'react';
import type {
  LayoutState,
  PanelState,
  TaskView,
  GridSubView,
  TaskFilters,
  HorizontalRailPosition,
  HorizontalRailTab,
} from '../lib/types.ts';

const STORAGE_KEY = 'soul-layout';

const DEFAULT_STATE: LayoutState = {
  // Legacy
  soulState: 'rail',
  chatState: 'open',
  taskState: 'open',
  taskView: 'kanban',
  gridSubView: 'grid',
  panelWidth: null,
  filters: { stage: 'all', priority: 'all', product: 'all' },
  // New layout
  activeProduct: null,
  railPosition: 'bottom',
  railExpanded: false,
  railHeightVh: 35,
  railTab: 'chat',
  chatSplitPct: 60,
  sessionsOpen: false,
  settingsOpen: false,
  autoInjectContext: true,
  showContextChip: true,
  toastsEnabled: true,
  inlineBadgesEnabled: true,
};

function loadState(): LayoutState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return DEFAULT_STATE;
    const parsed = JSON.parse(raw) as Partial<LayoutState>;
    return {
      ...DEFAULT_STATE,
      ...parsed,
      filters: { ...DEFAULT_STATE.filters, ...parsed.filters },
      // Always reset ephemeral UI state
      sessionsOpen: false,
      settingsOpen: false,
    };
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

  // ── Legacy setters ─────────────────────────────────────

  const canCollapse = useCallback(
    (panel: 'chat' | 'task'): boolean => {
      if (panel === 'chat') return state.taskState !== 'rail';
      return state.chatState !== 'rail';
    },
    [state.chatState, state.taskState],
  );

  const setSoulState = useCallback(
    (s: PanelState) => setState((prev) => ({ ...prev, soulState: s })),
    [setState],
  );

  const setChatState = useCallback(
    (s: PanelState) => {
      setState((prev) => {
        if (s === 'rail' && prev.taskState === 'rail') return prev;
        return { ...prev, chatState: s };
      });
    },
    [setState],
  );

  const setTaskState = useCallback(
    (s: PanelState) => {
      setState((prev) => {
        if (s === 'rail' && prev.chatState === 'rail') return prev;
        return { ...prev, taskState: s };
      });
    },
    [setState],
  );

  const setTaskView = useCallback(
    (v: TaskView) => setState((prev) => ({ ...prev, taskView: v, taskState: 'open' })),
    [setState],
  );

  const setGridSubView = useCallback(
    (v: GridSubView) => setState((prev) => ({ ...prev, gridSubView: v })),
    [setState],
  );

  const setPanelWidth = useCallback(
    (w: number | null) => setState((prev) => ({ ...prev, panelWidth: w })),
    [setState],
  );

  const setFilters = useCallback(
    (partial: Partial<TaskFilters>) =>
      setState((prev) => ({ ...prev, filters: { ...prev.filters, ...partial } })),
    [setState],
  );

  // ── New layout setters ─────────────────────────────────

  const setActiveProduct = useCallback(
    (product: string | null) => setState((prev) => ({ ...prev, activeProduct: product })),
    [setState],
  );

  const setRailPosition = useCallback(
    (pos: HorizontalRailPosition) => setState((prev) => ({ ...prev, railPosition: pos })),
    [setState],
  );

  const setRailExpanded = useCallback(
    (expanded: boolean) => setState((prev) => ({ ...prev, railExpanded: expanded })),
    [setState],
  );

  const setRailHeightVh = useCallback(
    (vh: number) => setState((prev) => ({ ...prev, railHeightVh: Math.min(60, Math.max(20, vh)) })),
    [setState],
  );

  const setRailTab = useCallback(
    (tab: HorizontalRailTab) =>
      setState((prev) => ({ ...prev, railTab: tab, railExpanded: true })),
    [setState],
  );

  const setChatSplitPct = useCallback(
    (pct: number) =>
      setState((prev) => ({ ...prev, chatSplitPct: Math.min(80, Math.max(30, pct)) })),
    [setState],
  );

  const setSessionsOpen = useCallback(
    (open: boolean) => setState((prev) => ({ ...prev, sessionsOpen: open })),
    [setState],
  );

  const setSettingsOpen = useCallback(
    (open: boolean) => setState((prev) => ({ ...prev, settingsOpen: open })),
    [setState],
  );

  const setAutoInjectContext = useCallback(
    (v: boolean) => setState((prev) => ({ ...prev, autoInjectContext: v })),
    [setState],
  );

  const setShowContextChip = useCallback(
    (v: boolean) => setState((prev) => ({ ...prev, showContextChip: v })),
    [setState],
  );

  const setToastsEnabled = useCallback(
    (v: boolean) => setState((prev) => ({ ...prev, toastsEnabled: v })),
    [setState],
  );

  const setInlineBadgesEnabled = useCallback(
    (v: boolean) => setState((prev) => ({ ...prev, inlineBadgesEnabled: v })),
    [setState],
  );

  return useMemo(
    () => ({
      ...state,
      // Legacy
      setSoulState,
      setChatState,
      setTaskState,
      setTaskView,
      setGridSubView,
      setPanelWidth,
      setFilters,
      canCollapse,
      // New
      setActiveProduct,
      setRailPosition,
      setRailExpanded,
      setRailHeightVh,
      setRailTab,
      setChatSplitPct,
      setSessionsOpen,
      setSettingsOpen,
      setAutoInjectContext,
      setShowContextChip,
      setToastsEnabled,
      setInlineBadgesEnabled,
    }),
    [
      state,
      setSoulState, setChatState, setTaskState, setTaskView,
      setGridSubView, setPanelWidth, setFilters, canCollapse,
      setActiveProduct, setRailPosition, setRailExpanded, setRailHeightVh,
      setRailTab, setChatSplitPct, setSessionsOpen, setSettingsOpen,
      setAutoInjectContext, setShowContextChip, setToastsEnabled, setInlineBadgesEnabled,
    ],
  );
}
