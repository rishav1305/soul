import { useState, useCallback, useMemo } from 'react';
import type {
  LayoutState,
  PanelState,
  TaskView,
  GridSubView,
  TaskFilters,
  HorizontalRailPosition,
  HorizontalRailTab,
  ChatPosition,
} from '../lib/types.ts';

const STORAGE_KEY = 'soul-layout';
const LAYOUT_V2_KEY = 'soul-layout-v2';

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

// Extended layout state for the redesign
interface LayoutV2State {
  activeProduct: string | null;
  chatPosition: ChatPosition;
  railExpanded: boolean;
  railHeight: number;
  chatSplit: number;
  autoInjectContext: boolean;
  showContextChip: boolean;
  toastsEnabled: boolean;
  inlineBadgesEnabled: boolean;
}

const DEFAULT_V2: LayoutV2State = {
  activeProduct: null,
  chatPosition: 'bottom',
  railExpanded: false,
  railHeight: Math.round(window.innerHeight * 0.25),
  chatSplit: 60,
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

function loadV2State(): LayoutV2State {
  try {
    const raw = localStorage.getItem(LAYOUT_V2_KEY);
    if (!raw) return DEFAULT_V2;
    const parsed = JSON.parse(raw) as Partial<LayoutV2State>;
    return { ...DEFAULT_V2, ...parsed };
  } catch {
    return DEFAULT_V2;
  }
}

function saveV2State(state: LayoutV2State): void {
  try {
    localStorage.setItem(LAYOUT_V2_KEY, JSON.stringify(state));
  } catch {}
}

export function useLayoutStore() {
  const [state, _setState] = useState<LayoutState>(loadState);
  const [v2, _setV2] = useState<LayoutV2State>(loadV2State);

  const setState = useCallback((updater: (prev: LayoutState) => LayoutState) => {
    _setState((prev) => {
      const next = updater(prev);
      saveState(next);
      return next;
    });
  }, []);

  const setV2 = useCallback((updater: (prev: LayoutV2State) => LayoutV2State) => {
    _setV2((prev) => {
      const next = updater(prev);
      saveV2State(next);
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

  // ── Legacy setters ─────────────────────────────────────

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

  // ── V2 setters ─────────────────────────────────────────

  const setChatPosition = useCallback(
    (p: ChatPosition) => setV2((prev) => ({ ...prev, chatPosition: p })),
    [setV2],
  );

  const setRailHeight = useCallback(
    (h: number) => setV2((prev) => ({ ...prev, railHeight: h })),
    [setV2],
  );

  const setChatSplit = useCallback(
    (s: number) => setV2((prev) => ({ ...prev, chatSplit: s })),
    [setV2],
  );

  return useMemo(
    () => ({
      // Legacy fields
      ...state,
      // V2 fields
      ...v2,
      // Legacy setters
      setSoulState,
      setChatState,
      setTaskState,
      setTaskView,
      setGridSubView,
      setPanelWidth,
      setFilters,
      canCollapse,
      // New layout setters
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
      // V2 setters
      setChatPosition,
      setRailHeight,
      setChatSplit,
    }),
    [
      state, v2,
      setSoulState, setChatState, setTaskState, setTaskView,
      setGridSubView, setPanelWidth, setFilters, canCollapse,
      setActiveProduct, setRailPosition, setRailExpanded, setRailHeightVh,
      setRailTab, setChatSplitPct, setSessionsOpen, setSettingsOpen,
      setAutoInjectContext, setShowContextChip, setToastsEnabled, setInlineBadgesEnabled,
      setChatPosition, setRailHeight, setChatSplit,
    ],
  );
}
