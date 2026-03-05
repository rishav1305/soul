import { useState, useCallback, useMemo } from 'react';
import type {
  LayoutState,
  TaskView,
  GridSubView,
  TaskFilters,
  HorizontalRailPosition,
  HorizontalRailTab,
} from '../lib/types.ts';

const STORAGE_KEY = 'soul-layout';

const DEFAULT_STATE: LayoutState = {
  taskView: 'kanban',
  gridSubView: 'grid',
  panelWidth: null,
  filters: { stage: 'all', priority: 'all', product: 'all' },
  activeProduct: null,
  railPosition: 'bottom',
  railExpanded: false,
  railHeightVh: 35,
  railTab: 'chat',
  chatSplitPct: 60,
  panelExpanded: false,
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
      panelExpanded: false,
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

export function useLayoutStore() {
  const [state, _setState] = useState<LayoutState>(loadState);

  const setState = useCallback((updater: (prev: LayoutState) => LayoutState) => {
    _setState((prev) => {
      const next = updater(prev);
      saveState(next);
      return next;
    });
  }, []);

  // ── Setters ─────────────────────────────────────────

  const setTaskView = useCallback(
    (v: TaskView) => setState((prev) => ({ ...prev, taskView: v })),
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

  const setPanelExpanded = useCallback(
    (v: boolean) => setState((prev) => ({ ...prev, panelExpanded: v })),
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
      setTaskView,
      setGridSubView,
      setPanelWidth,
      setFilters,
      setActiveProduct,
      setRailPosition,
      setRailExpanded,
      setRailHeightVh,
      setRailTab,
      setChatSplitPct,
      setPanelExpanded,
      setSessionsOpen,
      setSettingsOpen,
      setAutoInjectContext,
      setShowContextChip,
      setToastsEnabled,
      setInlineBadgesEnabled,
    }),
    [
      state,
      setTaskView, setGridSubView, setPanelWidth, setFilters,
      setActiveProduct, setRailPosition, setRailExpanded, setRailHeightVh,
      setRailTab, setChatSplitPct, setPanelExpanded, setSessionsOpen, setSettingsOpen,
      setAutoInjectContext, setShowContextChip, setToastsEnabled, setInlineBadgesEnabled,
    ],
  );
}
