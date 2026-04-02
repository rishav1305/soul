// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, act, cleanup } from '@testing-library/react';
import { useLayoutStore } from './useLayoutStore';

describe('useLayoutStore', () => {
  beforeEach(() => {
    localStorage.clear();
  });
  afterEach(() => cleanup());

  it('returns default state on first load', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.taskView).toBe('kanban');
    expect(result.current.chatSplitPct).toBe(60);
    expect(result.current.railPosition).toBe('bottom');
    expect(result.current.filters).toEqual({ stage: 'all', priority: 'all', product: 'all' });
  });

  it('setTaskView updates taskView', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setTaskView('list'));
    expect(result.current.taskView).toBe('list');
  });

  it('persists state to localStorage', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setTaskView('grid'));

    const stored = JSON.parse(localStorage.getItem('soul-layout')!);
    expect(stored.taskView).toBe('grid');
  });

  it('loads persisted state on mount', () => {
    // Pre-populate localStorage with correct version
    localStorage.setItem('soul-layout-version', '4');
    localStorage.setItem('soul-layout', JSON.stringify({ taskView: 'list' }));

    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.taskView).toBe('list');
  });

  it('resets to defaults when layout version is outdated', () => {
    localStorage.setItem('soul-layout-version', '1'); // old version
    localStorage.setItem('soul-layout', JSON.stringify({ taskView: 'list' }));

    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.taskView).toBe('kanban'); // reset to default
  });

  it('setFilters merges partial filters', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setFilters({ stage: 'active' }));
    expect(result.current.filters.stage).toBe('active');
    expect(result.current.filters.priority).toBe('all'); // untouched
  });

  it('setActiveProduct updates product', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setActiveProduct('chat'));
    expect(result.current.activeProduct).toBe('chat');
  });

  it('setChatSplitPct clamps between 30 and 80', () => {
    const { result } = renderHook(() => useLayoutStore());

    act(() => result.current.setChatSplitPct(10));
    expect(result.current.chatSplitPct).toBe(30);

    act(() => result.current.setChatSplitPct(90));
    expect(result.current.chatSplitPct).toBe(80);

    act(() => result.current.setChatSplitPct(55));
    expect(result.current.chatSplitPct).toBe(55);
  });

  it('setRailHeightVh clamps between 20 and 60', () => {
    const { result } = renderHook(() => useLayoutStore());

    act(() => result.current.setRailHeightVh(10));
    expect(result.current.railHeightVh).toBe(20);

    act(() => result.current.setRailHeightVh(70));
    expect(result.current.railHeightVh).toBe(60);
  });

  it('setRailTab also expands the rail', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.railExpanded).toBe(false);

    act(() => result.current.setRailTab('tasks'));
    expect(result.current.railTab).toBe('tasks');
    expect(result.current.railExpanded).toBe(true);
  });

  it('setSessionsOpen toggles sessions', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setSessionsOpen(true));
    expect(result.current.sessionsOpen).toBe(true);
  });

  it('sessionsOpen resets to false on load', () => {
    localStorage.setItem('soul-layout-version', '4');
    localStorage.setItem('soul-layout', JSON.stringify({ sessionsOpen: true }));

    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.sessionsOpen).toBe(false); // always reset
  });

  it('setDrawerLayout updates layout', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setDrawerLayout('split'));
    expect(result.current.drawerLayout).toBe('split');
  });

  it('setSyncProductFilter toggles sync', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setSyncProductFilter(true));
    expect(result.current.syncProductFilter).toBe(true);
  });

  it('handles corrupted localStorage gracefully', () => {
    localStorage.setItem('soul-layout-version', '4');
    localStorage.setItem('soul-layout', 'not-json');

    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.taskView).toBe('kanban'); // default
  });

  it('setGridSubView updates gridSubView', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setGridSubView('priority'));
    expect(result.current.gridSubView).toBe('priority');
  });

  it('setPanelWidth updates panelWidth', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.panelWidth).toBeNull();
    act(() => result.current.setPanelWidth(400));
    expect(result.current.panelWidth).toBe(400);
  });

  it('setPanelWidth accepts null', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setPanelWidth(500));
    act(() => result.current.setPanelWidth(null));
    expect(result.current.panelWidth).toBeNull();
  });

  it('setRailPosition updates railPosition', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setRailPosition('top'));
    expect(result.current.railPosition).toBe('top');
  });

  it('setChatPosition updates chatPosition', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setChatPosition('bottom'));
    expect(result.current.chatPosition).toBe('bottom');
  });

  it('setTasksPosition updates tasksPosition', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setTasksPosition('top'));
    expect(result.current.tasksPosition).toBe('top');
  });

  it('setRailExpanded updates railExpanded', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.railExpanded).toBe(false);
    act(() => result.current.setRailExpanded(true));
    expect(result.current.railExpanded).toBe(true);
  });

  it('setPanelExpanded updates panelExpanded', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.panelExpanded).toBe(true);
    act(() => result.current.setPanelExpanded(false));
    expect(result.current.panelExpanded).toBe(false);
  });

  it('setSettingsOpen toggles settings', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.settingsOpen).toBe(false);
    act(() => result.current.setSettingsOpen(true));
    expect(result.current.settingsOpen).toBe(true);
  });

  it('settingsOpen resets to false on load', () => {
    localStorage.setItem('soul-layout-version', '4');
    localStorage.setItem('soul-layout', JSON.stringify({ settingsOpen: true }));

    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.settingsOpen).toBe(false);
  });

  it('setAutoInjectContext toggles', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.autoInjectContext).toBe(true);
    act(() => result.current.setAutoInjectContext(false));
    expect(result.current.autoInjectContext).toBe(false);
  });

  it('setShowContextChip toggles', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.showContextChip).toBe(true);
    act(() => result.current.setShowContextChip(false));
    expect(result.current.showContextChip).toBe(false);
  });

  it('setToastsEnabled toggles', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.toastsEnabled).toBe(true);
    act(() => result.current.setToastsEnabled(false));
    expect(result.current.toastsEnabled).toBe(false);
  });

  it('setInlineBadgesEnabled toggles', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.inlineBadgesEnabled).toBe(true);
    act(() => result.current.setInlineBadgesEnabled(false));
    expect(result.current.inlineBadgesEnabled).toBe(false);
  });

  it('setChatRailExpanded toggles', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.chatRailExpanded).toBe(false);
    act(() => result.current.setChatRailExpanded(true));
    expect(result.current.chatRailExpanded).toBe(true);
  });

  it('setChatRailHeightVh clamps between 20 and 60', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setChatRailHeightVh(5));
    expect(result.current.chatRailHeightVh).toBe(20);
    act(() => result.current.setChatRailHeightVh(80));
    expect(result.current.chatRailHeightVh).toBe(60);
    act(() => result.current.setChatRailHeightVh(45));
    expect(result.current.chatRailHeightVh).toBe(45);
  });

  it('setTasksRailExpanded toggles', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.tasksRailExpanded).toBe(false);
    act(() => result.current.setTasksRailExpanded(true));
    expect(result.current.tasksRailExpanded).toBe(true);
  });

  it('setTasksRailHeightVh clamps between 20 and 60', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setTasksRailHeightVh(10));
    expect(result.current.tasksRailHeightVh).toBe(20);
    act(() => result.current.setTasksRailHeightVh(75));
    expect(result.current.tasksRailHeightVh).toBe(60);
  });

  it('setRightChatExpanded toggles', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.rightChatExpanded).toBe(true);
    act(() => result.current.setRightChatExpanded(false));
    expect(result.current.rightChatExpanded).toBe(false);
  });

  it('setRightTasksExpanded toggles', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.rightTasksExpanded).toBe(true);
    act(() => result.current.setRightTasksExpanded(false));
    expect(result.current.rightTasksExpanded).toBe(false);
  });

  it('setRightPanelWidth clamps to minimum 280', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setRightPanelWidth(100));
    expect(result.current.rightPanelWidth).toBe(280);
  });

  it('setRightChatWidth clamps to minimum 280', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setRightChatWidth(50));
    expect(result.current.rightChatWidth).toBe(280);
  });

  it('setRightTasksWidth clamps to minimum 280', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setRightTasksWidth(0));
    expect(result.current.rightTasksWidth).toBe(280);
  });

  it('default state has correct boolean defaults', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.autoInjectContext).toBe(true);
    expect(result.current.showContextChip).toBe(true);
    expect(result.current.toastsEnabled).toBe(true);
    expect(result.current.inlineBadgesEnabled).toBe(true);
    expect(result.current.syncProductFilter).toBe(false);
    expect(result.current.chatRailExpanded).toBe(false);
    expect(result.current.tasksRailExpanded).toBe(false);
    expect(result.current.rightChatExpanded).toBe(true);
    expect(result.current.rightTasksExpanded).toBe(true);
  });

  it('default numeric values', () => {
    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.railHeightVh).toBe(35);
    expect(result.current.chatRailHeightVh).toBe(35);
    expect(result.current.tasksRailHeightVh).toBe(35);
    expect(result.current.rightPanelWidth).toBe(720);
    expect(result.current.rightChatWidth).toBe(600);
    expect(result.current.rightTasksWidth).toBe(600);
  });

  it('setFilters updates multiple keys', () => {
    const { result } = renderHook(() => useLayoutStore());
    act(() => result.current.setFilters({ stage: 'active', priority: 'high' }));
    expect(result.current.filters.stage).toBe('active');
    expect(result.current.filters.priority).toBe('high');
    expect(result.current.filters.product).toBe('all');
  });

  it('localStorage save failure does not crash', () => {
    const { result } = renderHook(() => useLayoutStore());
    // Override setItem to throw
    const orig = localStorage.setItem.bind(localStorage);
    vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => {
      throw new Error('QuotaExceededError');
    });
    // Should not throw
    act(() => result.current.setTaskView('grid'));
    expect(result.current.taskView).toBe('grid');
    vi.restoreAllMocks();
  });

  it('loads persisted filters merged with defaults', () => {
    localStorage.setItem('soul-layout-version', '4');
    localStorage.setItem('soul-layout', JSON.stringify({ filters: { stage: 'done' } }));

    const { result } = renderHook(() => useLayoutStore());
    expect(result.current.filters.stage).toBe('done');
    expect(result.current.filters.priority).toBe('all');
    expect(result.current.filters.product).toBe('all');
  });
});
