import { useState, useCallback, useMemo } from 'react';
import type {
  LayoutState,
  TaskView,
  GridSubView,
  TaskFilters,
  HorizontalRailPosition,
  PanelPosition,
  HorizontalRailTab,
  DrawerLayout,
} from '../lib/types.ts';

const STORAGE_KEY = 'soul-layout';

const DEFAULT_STATE: LayoutState = {
  taskView: 'kanban',
  gridSubView: 'grid',
  panelWidth: null,
  filters: { stage: 'all', priority: 'all', product: 'all' },
  activeProduct: null,
  railPosition: 'bottom',
  chatPosition: 'right',
  tasksPosition: 'right',
  railExpanded: false,
  railHeightVh: 35,
  railTab: 'chat',
  chatSplitPct: 60,
  drawerLayout: 'split',
  panelExpanded: false,
  sessionsOpen: false,
  settingsOpen: false,
  autoInjectContext: true,
  showContextChip: true,
  toastsEnabled: true,
  inlineBadgesEnabled: true,
  syncProductFilter: false,
  chatRailExpanded: false,
  chatRailHeightVh: 35,
  tasksRailExpanded: false,
  tasksRailHeightVh: 35,
  rightPanelWidth: 480,
  rightChatWidth: 420,
  rightTasksWidth: 420,
  rightChatExpanded: true,
  rightTasksExpanded: true,
};

// Bump this when layout defaults change to force migration for existing users.
const LAYOUT_VERSION = 2; // v2: chat+tasks default to right panel
const VERSION_KEY = 'soul-layout-version';

function loadState(): LayoutState {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    const storedVersion = Number(localStorage.getItem(VERSION_KEY) || '0');
    if (!raw || storedVersion < LAYOUT_VERSION) {
      // Force new defaults when version bumps
      localStorage.setItem(VERSION_KEY, String(LAYOUT_VERSION));
      localStorage.removeItem(STORAGE_KEY);
      return DEFAULT_STATE;
    }
    const parsed = JSON.parse(raw) as Partial<LayoutState>;
    return {
      ...DEFAULT_STATE,
      ...parsed,
      filters: { ...DEFAULT_STATE.filters, ...parsed.filters },
      // Reset overlays (not panel layout)
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

  const setChatPosition = useCallback(
    (pos: PanelPosition) => setState((prev) => ({ ...prev, chatPosition: pos })),
    [setState],
  );

  const setTasksPosition = useCallback(
    (pos: PanelPosition) => setState((prev) => ({ ...prev, tasksPosition: pos })),
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

  const setDrawerLayout = useCallback(
    (v: DrawerLayout) => setState((prev) => ({ ...prev, drawerLayout: v })),
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

  const setSyncProductFilter = useCallback(
    (v: boolean) => setState((prev) => ({ ...prev, syncProductFilter: v })),
    [setState],
  );

  const setChatRailExpanded = useCallback(
    (v: boolean) => setState((prev) => ({ ...prev, chatRailExpanded: v })),
    [setState],
  );

  const setChatRailHeightVh = useCallback(
    (vh: number) => setState((prev) => ({ ...prev, chatRailHeightVh: Math.min(60, Math.max(20, vh)) })),
    [setState],
  );

  const setTasksRailExpanded = useCallback(
    (v: boolean) => setState((prev) => ({ ...prev, tasksRailExpanded: v })),
    [setState],
  );

  const setTasksRailHeightVh = useCallback(
    (vh: number) => setState((prev) => ({ ...prev, tasksRailHeightVh: Math.min(60, Math.max(20, vh)) })),
    [setState],
  );

  const clampWidth = (w: number) => Math.min(Math.round(window.innerWidth * 0.7), Math.max(280, w));

  const setRightPanelWidth = useCallback(
    (w: number) => setState((prev) => ({ ...prev, rightPanelWidth: clampWidth(w) })),
    [setState],
  );

  const setRightChatWidth = useCallback(
    (w: number) => setState((prev) => ({ ...prev, rightChatWidth: clampWidth(w) })),
    [setState],
  );

  const setRightTasksWidth = useCallback(
    (w: number) => setState((prev) => ({ ...prev, rightTasksWidth: clampWidth(w) })),
    [setState],
  );

  const setRightChatExpanded = useCallback(
    (v: boolean) => setState((prev) => ({ ...prev, rightChatExpanded: v })),
    [setState],
  );

  const setRightTasksExpanded = useCallback(
    (v: boolean) => setState((prev) => ({ ...prev, rightTasksExpanded: v })),
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
      setChatPosition,
      setTasksPosition,
      setRailExpanded,
      setRailHeightVh,
      setRailTab,
      setChatSplitPct,
      setDrawerLayout,
      setPanelExpanded,
      setSessionsOpen,
      setSettingsOpen,
      setAutoInjectContext,
      setShowContextChip,
      setToastsEnabled,
      setInlineBadgesEnabled,
      setSyncProductFilter,
      setChatRailExpanded,
      setChatRailHeightVh,
      setTasksRailExpanded,
      setTasksRailHeightVh,
      setRightPanelWidth,
      setRightChatWidth,
      setRightTasksWidth,
      setRightChatExpanded,
      setRightTasksExpanded,
    }),
    [
      state,
      setTaskView, setGridSubView, setPanelWidth, setFilters,
      setActiveProduct, setRailPosition, setChatPosition, setTasksPosition, setRailExpanded, setRailHeightVh,
      setRailTab, setChatSplitPct, setDrawerLayout, setPanelExpanded, setSessionsOpen, setSettingsOpen,
      setAutoInjectContext, setShowContextChip, setToastsEnabled, setInlineBadgesEnabled, setSyncProductFilter,
      setChatRailExpanded, setChatRailHeightVh, setTasksRailExpanded, setTasksRailHeightVh,
      setRightPanelWidth, setRightChatWidth, setRightTasksWidth, setRightChatExpanded, setRightTasksExpanded,
    ],
  );
}
