// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import { MemorySearch } from './MemorySearch';
import type { MemoryResult, MemoryQuery } from './MemorySearch';

function makeResult(overrides: Partial<MemoryResult> = {}): MemoryResult {
  return {
    doc_id: 'shuri/project_soulgraph.md',
    content: 'SoulGraph migration plan details here',
    metadata: {
      agent: 'shuri',
      type: 'project',
      name: 'SoulGraph Migration',
      description: 'CEO committed to Phase 0',
      source_path: '/home/rishav/.claude/agent-memory/shuri/project_soulgraph.md',
      updated_at: '2026-03-30T16:00:00',
    },
    score: 0.92,
    ...overrides,
  };
}

describe('MemorySearch', () => {
  afterEach(() => cleanup());

  it('renders container', () => {
    render(<MemorySearch onSearch={vi.fn().mockResolvedValue([])} />);
    expect(screen.getByTestId('memory-search')).toBeTruthy();
  });

  it('renders search input', () => {
    render(<MemorySearch onSearch={vi.fn().mockResolvedValue([])} />);
    expect(screen.getByTestId('memory-search-input')).toBeTruthy();
  });

  it('renders search button', () => {
    render(<MemorySearch onSearch={vi.fn().mockResolvedValue([])} />);
    expect(screen.getByTestId('memory-search-submit')).toBeTruthy();
  });

  it('search button disabled when input empty', () => {
    render(<MemorySearch onSearch={vi.fn().mockResolvedValue([])} />);
    expect((screen.getByTestId('memory-search-submit') as HTMLButtonElement).disabled).toBe(true);
  });

  it('search button enabled when input has text', () => {
    render(<MemorySearch onSearch={vi.fn().mockResolvedValue([])} />);
    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'migration' } });
    expect((screen.getByTestId('memory-search-submit') as HTMLButtonElement).disabled).toBe(false);
  });

  it('renders collection filter', () => {
    render(<MemorySearch onSearch={vi.fn().mockResolvedValue([])} />);
    expect(screen.getByTestId('memory-collection-filter')).toBeTruthy();
  });

  it('renders agent filter with agents list', () => {
    render(<MemorySearch onSearch={vi.fn().mockResolvedValue([])} agents={['shuri', 'happy', 'pepper']} />);
    const select = screen.getByTestId('memory-agent-filter') as HTMLSelectElement;
    expect(select).toBeTruthy();
    // All Agents + 3 agents = 4 options
    expect(select.options.length).toBe(4);
  });

  it('renders type filter', () => {
    render(<MemorySearch onSearch={vi.fn().mockResolvedValue([])} />);
    expect(screen.getByTestId('memory-type-filter')).toBeTruthy();
  });

  it('renders top-k filter', () => {
    render(<MemorySearch onSearch={vi.fn().mockResolvedValue([])} />);
    expect(screen.getByTestId('memory-topk-filter')).toBeTruthy();
  });

  it('calls onSearch with correct query when submitted', async () => {
    const onSearch = vi.fn().mockResolvedValue([]);
    render(<MemorySearch onSearch={onSearch} />);

    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'SoulGraph migration' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });

    expect(onSearch).toHaveBeenCalledWith({
      collection: 'soul_agent_memory',
      query: 'SoulGraph migration',
      top_k: 5,
    });
  });

  it('includes agent_filter when set', async () => {
    const onSearch = vi.fn().mockResolvedValue([]);
    render(<MemorySearch onSearch={onSearch} agents={['shuri']} />);

    fireEvent.change(screen.getByTestId('memory-agent-filter'), { target: { value: 'shuri' } });
    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });

    const query = onSearch.mock.calls[0][0] as MemoryQuery;
    expect(query.agent_filter).toBe('shuri');
  });

  it('includes type_filter when set', async () => {
    const onSearch = vi.fn().mockResolvedValue([]);
    render(<MemorySearch onSearch={onSearch} />);

    fireEvent.change(screen.getByTestId('memory-type-filter'), { target: { value: 'project' } });
    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });

    const query = onSearch.mock.calls[0][0] as MemoryQuery;
    expect(query.type_filter).toBe('project');
  });

  it('shows empty state after search with no results', async () => {
    const onSearch = vi.fn().mockResolvedValue([]);
    render(<MemorySearch onSearch={onSearch} />);

    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'nothing here' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });

    expect(screen.getByTestId('memory-search-empty')).toBeTruthy();
  });

  it('does not show empty state before search', () => {
    render(<MemorySearch onSearch={vi.fn().mockResolvedValue([])} />);
    expect(screen.queryByTestId('memory-search-empty')).toBeNull();
  });

  it('renders results after successful search', async () => {
    const results = [makeResult()];
    const onSearch = vi.fn().mockResolvedValue(results);
    render(<MemorySearch onSearch={onSearch} />);

    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'SoulGraph' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });

    expect(screen.getByTestId('memory-result-shuri/project_soulgraph.md')).toBeTruthy();
  });

  it('shows result name', async () => {
    const results = [makeResult()];
    const onSearch = vi.fn().mockResolvedValue(results);
    render(<MemorySearch onSearch={onSearch} />);

    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });

    expect(screen.getByTestId('memory-result-name').textContent).toBe('SoulGraph Migration');
  });

  it('shows result agent badge', async () => {
    const results = [makeResult()];
    const onSearch = vi.fn().mockResolvedValue(results);
    render(<MemorySearch onSearch={onSearch} />);

    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });

    expect(screen.getByTestId('memory-result-agent').textContent).toBe('shuri');
  });

  it('shows result type badge', async () => {
    const results = [makeResult()];
    const onSearch = vi.fn().mockResolvedValue(results);
    render(<MemorySearch onSearch={onSearch} />);

    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });

    expect(screen.getByTestId('memory-result-type').textContent).toBe('project');
  });

  it('shows result score as percentage', async () => {
    const results = [makeResult({ score: 0.92 })];
    const onSearch = vi.fn().mockResolvedValue(results);
    render(<MemorySearch onSearch={onSearch} />);

    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });

    expect(screen.getByTestId('memory-result-score').textContent).toBe('92%');
  });

  it('shows result content preview', async () => {
    const results = [makeResult()];
    const onSearch = vi.fn().mockResolvedValue(results);
    render(<MemorySearch onSearch={onSearch} />);

    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });

    expect(screen.getByTestId('memory-result-content').textContent).toContain('SoulGraph migration plan');
  });

  it('shows result file path', async () => {
    const results = [makeResult()];
    const onSearch = vi.fn().mockResolvedValue(results);
    render(<MemorySearch onSearch={onSearch} />);

    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });

    expect(screen.getByTestId('memory-result-path').textContent).toContain('project_soulgraph.md');
  });

  it('shows error message', () => {
    render(<MemorySearch onSearch={vi.fn().mockResolvedValue([])} error="ChromaDB is down" />);
    expect(screen.getByTestId('memory-search-error').textContent).toBe('ChromaDB is down');
  });

  it('hides error when null', () => {
    render(<MemorySearch onSearch={vi.fn().mockResolvedValue([])} error={null} />);
    expect(screen.queryByTestId('memory-search-error')).toBeNull();
  });

  it('shows loading state on button', () => {
    render(<MemorySearch onSearch={vi.fn().mockResolvedValue([])} loading={true} />);
    expect(screen.getByTestId('memory-search-submit').textContent).toBe('Searching...');
  });

  it('search button disabled when loading', () => {
    render(<MemorySearch onSearch={vi.fn().mockResolvedValue([])} loading={true} />);
    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'test' } });
    expect((screen.getByTestId('memory-search-submit') as HTMLButtonElement).disabled).toBe(true);
  });

  it('searches on Enter key', async () => {
    const onSearch = vi.fn().mockResolvedValue([]);
    render(<MemorySearch onSearch={onSearch} />);

    const input = screen.getByTestId('memory-search-input');
    fireEvent.change(input, { target: { value: 'enter test' } });
    await act(async () => {
      fireEvent.keyDown(input, { key: 'Enter' });
    });

    expect(onSearch).toHaveBeenCalled();
  });

  it('uses selected collection in query', async () => {
    const onSearch = vi.fn().mockResolvedValue([]);
    render(<MemorySearch onSearch={onSearch} />);

    fireEvent.change(screen.getByTestId('memory-collection-filter'), { target: { value: 'soul_shared_kb' } });
    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });

    const query = onSearch.mock.calls[0][0] as MemoryQuery;
    expect(query.collection).toBe('soul_shared_kb');
  });

  it('falls back to id when name is missing', async () => {
    const result = makeResult({
      doc_id: 'some-doc-id',
      metadata: { ...makeResult().metadata, name: undefined },
    });
    const onSearch = vi.fn().mockResolvedValue([result]);
    render(<MemorySearch onSearch={onSearch} />);

    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });

    expect(screen.getByTestId('memory-result-name').textContent).toBe('some-doc-id');
  });

  it('renders multiple results', async () => {
    const results = [
      makeResult({ doc_id: 'doc-1', score: 0.9 }),
      makeResult({ doc_id: 'doc-2', score: 0.7 }),
    ];
    const onSearch = vi.fn().mockResolvedValue(results);
    render(<MemorySearch onSearch={onSearch} />);

    fireEvent.change(screen.getByTestId('memory-search-input'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('memory-search-submit'));
    });

    expect(screen.getByTestId('memory-result-doc-1')).toBeTruthy();
    expect(screen.getByTestId('memory-result-doc-2')).toBeTruthy();
  });
});
