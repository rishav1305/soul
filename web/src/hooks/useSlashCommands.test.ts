// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, waitFor, cleanup } from '@testing-library/react';
import { useSlashCommands } from './useSlashCommands';

// Mock authFetch
const mockAuthFetch = vi.fn();
vi.mock('../lib/api.ts', () => ({
  authFetch: (...args: unknown[]) => mockAuthFetch(...args),
}));

describe('useSlashCommands', () => {
  beforeEach(() => {
    mockAuthFetch.mockReset();
  });
  afterEach(() => cleanup());

  it('returns builtin commands immediately', () => {
    mockAuthFetch.mockReturnValue(new Promise(() => {})); // pending
    const { result } = renderHook(() => useSlashCommands());
    expect(result.current.length).toBeGreaterThanOrEqual(7);
    expect(result.current.find(c => c.name === 'think')).toBeTruthy();
    expect(result.current.find(c => c.name === 'brainstorm')).toBeTruthy();
    expect(result.current.find(c => c.name === 'code')).toBeTruthy();
    expect(result.current.find(c => c.name === 'debug')).toBeTruthy();
  });

  it('fetches skills from /api/skills', async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ skills: [{ name: 'deploy' }] }),
    });

    const { result } = renderHook(() => useSlashCommands());

    await waitFor(() => {
      expect(result.current.find(c => c.name === 'deploy')).toBeTruthy();
    });

    expect(mockAuthFetch).toHaveBeenCalledWith('/api/skills');
  });

  it('merges skills with builtins without duplicates', async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ skills: [
        { name: 'think' }, // duplicate of builtin — should be filtered
        { name: 'custom-tool' },
      ]}),
    });

    const { result } = renderHook(() => useSlashCommands());

    await waitFor(() => {
      expect(result.current.find(c => c.name === 'custom-tool')).toBeTruthy();
    });

    // 'think' should appear only once
    const thinkCommands = result.current.filter(c => c.name === 'think');
    expect(thinkCommands.length).toBe(1);
    expect(thinkCommands[0].builtin).toBe(true);
  });

  it('keeps builtins on fetch failure', async () => {
    mockAuthFetch.mockRejectedValue(new Error('fetch failed'));

    const { result } = renderHook(() => useSlashCommands());

    // Wait a tick for the effect to settle
    await waitFor(() => {
      // Should still have builtins
      expect(result.current.find(c => c.name === 'code')).toBeTruthy();
    });

    expect(result.current.length).toBe(7); // only builtins
  });

  it('handles non-ok response by keeping builtins', async () => {
    mockAuthFetch.mockResolvedValue({
      ok: false,
      status: 500,
    });

    const { result } = renderHook(() => useSlashCommands());

    await waitFor(() => {
      expect(result.current.length).toBe(7);
    });
  });

  it('handles array response format', async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve([{ name: 'skill-a' }, { name: 'skill-b' }]),
    });

    const { result } = renderHook(() => useSlashCommands());

    await waitFor(() => {
      expect(result.current.find(c => c.name === 'skill-a')).toBeTruthy();
      expect(result.current.find(c => c.name === 'skill-b')).toBeTruthy();
    });
  });

  it('generates chatType from skill name', async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ skills: [{ name: 'deploy' }] }),
    });

    const { result } = renderHook(() => useSlashCommands());

    await waitFor(() => {
      const deploy = result.current.find(c => c.name === 'deploy');
      expect(deploy?.chatType).toBe('Deploy');
    });
  });

  it('filters duplicate builtins case-insensitively', async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ skills: [
        { name: 'Think' }, // uppercase version of builtin
        { name: 'CODE' },  // all-caps version
      ]}),
    });

    const { result } = renderHook(() => useSlashCommands());

    await waitFor(() => {
      // Should still only have builtin versions
      const thinkResults = result.current.filter(c => c.name.toLowerCase() === 'think');
      expect(thinkResults.length).toBe(1);
      expect(thinkResults[0].builtin).toBe(true);

      const codeResults = result.current.filter(c => c.name.toLowerCase() === 'code');
      expect(codeResults.length).toBe(1);
      expect(codeResults[0].builtin).toBeUndefined(); // builtin code has no .builtin flag, but chatType='Code'
    });
  });

  it('skill commands have description format "{name} skill"', async () => {
    mockAuthFetch.mockResolvedValue({
      ok: true,
      json: () => Promise.resolve({ skills: [{ name: 'monitor' }] }),
    });

    const { result } = renderHook(() => useSlashCommands());

    await waitFor(() => {
      const monitor = result.current.find(c => c.name === 'monitor');
      expect(monitor?.description).toBe('monitor skill');
    });
  });

  it('all 7 builtin commands are present by name', () => {
    mockAuthFetch.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useSlashCommands());

    const names = result.current.map(c => c.name);
    expect(names).toContain('think');
    expect(names).toContain('brainstorm');
    expect(names).toContain('code');
    expect(names).toContain('debug');
    expect(names).toContain('architect');
    expect(names).toContain('review');
    expect(names).toContain('tdd');
    expect(result.current.length).toBe(7);
  });

  it('builtin think command has builtin flag', () => {
    mockAuthFetch.mockReturnValue(new Promise(() => {}));
    const { result } = renderHook(() => useSlashCommands());
    const think = result.current.find(c => c.name === 'think');
    expect(think?.builtin).toBe(true);
  });
});
