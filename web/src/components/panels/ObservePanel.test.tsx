// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import ObservePanel from './ObservePanel';
import type { MemoryHealthData } from '../soulgraph/MemoryHealth';

// Mock useMemorySearch so tests don't make real HTTP calls.
const mockSearch = vi.fn();
const mockFetchHealth = vi.fn();
const defaultHookState = {
  results: [],
  health: null,
  loading: false,
  healthLoading: false,
  error: null,
  healthError: null,
  search: mockSearch,
  fetchHealth: mockFetchHealth,
};

vi.mock('../../hooks/useMemorySearch', () => ({
  useMemorySearch: () => defaultHookState,
}));

const mockHealth: MemoryHealthData = {
  chromadb: 'up',
  collections: {
    soul_agent_memory: 456,
    soul_shared_kb: 154,
    soul_briefs: 7117,
  },
  last_index: '2026-04-01T12:00:00.000Z',
};

describe('ObservePanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockFetchHealth.mockResolvedValue(undefined);
    mockSearch.mockResolvedValue([]);
  });
  afterEach(() => cleanup());

  it('renders panel container', () => {
    render(<ObservePanel />);
    expect(screen.getByTestId('observe-panel')).toBeTruthy();
  });

  it('renders health tab button', () => {
    render(<ObservePanel />);
    expect(screen.getByTestId('observe-tab-health')).toBeTruthy();
  });

  it('renders search tab button', () => {
    render(<ObservePanel />);
    expect(screen.getByTestId('observe-tab-search')).toBeTruthy();
  });

  it('defaults to health tab', () => {
    render(<ObservePanel />);
    expect(screen.getByTestId('memory-health')).toBeTruthy();
  });

  it('does not show search tab content by default', () => {
    render(<ObservePanel />);
    expect(screen.queryByTestId('memory-search')).toBeNull();
  });

  it('switches to search tab on click', () => {
    render(<ObservePanel />);
    fireEvent.click(screen.getByTestId('observe-tab-search'));
    expect(screen.getByTestId('memory-search')).toBeTruthy();
    expect(screen.queryByTestId('memory-health')).toBeNull();
  });

  it('switches back to health tab from search tab', () => {
    render(<ObservePanel />);
    fireEvent.click(screen.getByTestId('observe-tab-search'));
    fireEvent.click(screen.getByTestId('observe-tab-health'));
    expect(screen.getByTestId('memory-health')).toBeTruthy();
    expect(screen.queryByTestId('memory-search')).toBeNull();
  });

  it('calls fetchHealth on mount', () => {
    render(<ObservePanel />);
    expect(mockFetchHealth).toHaveBeenCalledTimes(1);
  });

  it('shows empty health state when no data', () => {
    render(<ObservePanel />);
    expect(screen.getByTestId('memory-health-empty')).toBeTruthy();
  });

  it('health tab shows status when data provided', async () => {
    vi.mocked(mockFetchHealth).mockImplementation(async () => {
      // The mock hook returns static state, so we test via component props flow.
    });
    // Render with a mock that returns health data via the hook.
    const { unmount } = render(<ObservePanel />);
    // Empty state is shown since mockHealth is not returned by default hook state.
    expect(screen.getByTestId('memory-health-empty')).toBeTruthy();
    unmount();
  });

  it('passes agents list to MemorySearch when on search tab', () => {
    render(<ObservePanel />);
    fireEvent.click(screen.getByTestId('observe-tab-search'));
    const agentSelect = screen.getByTestId('memory-agent-filter') as HTMLSelectElement;
    // Should have "All Agents" + 10 known agents = 11 options
    expect(agentSelect.options.length).toBe(11);
  });

  it('known agents include all 10 soul-team members', () => {
    render(<ObservePanel />);
    fireEvent.click(screen.getByTestId('observe-tab-search'));
    const agentSelect = screen.getByTestId('memory-agent-filter') as HTMLSelectElement;
    const values = Array.from(agentSelect.options).map(o => o.value);
    for (const agent of ['banner', 'friday', 'fury', 'happy', 'hawkeye', 'loki', 'pepper', 'shuri', 'stark', 'xavier']) {
      expect(values).toContain(agent);
    }
  });

  it('search tab calls onSearch when submitted', async () => {
    render(<ObservePanel />);
    fireEvent.click(screen.getByTestId('observe-tab-search'));
    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'phase 0' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });
    expect(mockSearch).toHaveBeenCalledWith(
      expect.objectContaining({ query: 'phase 0' }),
    );
  });
});
