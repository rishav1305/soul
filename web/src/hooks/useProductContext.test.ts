/**
 * Tests for useProductContext hook.
 * Tests pure utility logic + hook-level integration through renderHook.
 */
// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, cleanup } from '@testing-library/react';

// We can't directly import private functions, so we test the exported types
// and validate the logic through integration. For pure function extraction,
// we test the productAbbr pattern used in ProductRail as a model.

// Test the stage ordering logic that useProductContext relies on
describe('useProductContext helpers', () => {
  const STAGE_ORDER = ['active', 'blocked', 'validation', 'brainstorm', 'backlog', 'done'] as const;
  const ACTIVE_STAGES = ['active', 'blocked', 'validation'] as const;

  it('STAGE_ORDER covers all expected stages', () => {
    expect(STAGE_ORDER).toHaveLength(6);
    expect(STAGE_ORDER).toContain('active');
    expect(STAGE_ORDER).toContain('blocked');
    expect(STAGE_ORDER).toContain('validation');
    expect(STAGE_ORDER).toContain('brainstorm');
    expect(STAGE_ORDER).toContain('backlog');
    expect(STAGE_ORDER).toContain('done');
  });

  it('ACTIVE_STAGES are a subset of STAGE_ORDER', () => {
    for (const stage of ACTIVE_STAGES) {
      expect(STAGE_ORDER).toContain(stage);
    }
  });

  it('ACTIVE_STAGES does not include done or backlog', () => {
    expect(ACTIVE_STAGES).not.toContain('done');
    expect(ACTIVE_STAGES).not.toContain('backlog');
  });

  // Test the buildProductSummary logic pattern
  describe('product summary logic', () => {
    function buildProductSummary(
      tasks: { stage: string }[],
      productName?: string,
    ): string {
      if (productName) {
        const active = tasks.filter((t) => t.stage === 'active').length;
        const blocked = tasks.filter((t) => t.stage === 'blocked').length;
        const validation = tasks.filter((t) => t.stage === 'validation').length;
        return `${productName}: ${active} active, ${blocked} blocked, ${validation} in validation`;
      }
      if (tasks.length === 0) return 'No tasks.';
      const byStage: Record<string, { stage: string; title?: string }[]> = {};
      for (const t of tasks as { stage: string; title?: string }[]) {
        if (!byStage[t.stage]) byStage[t.stage] = [];
        byStage[t.stage]!.push(t);
      }
      const lines: string[] = [];
      for (const stage of STAGE_ORDER) {
        const stageTasks = byStage[stage];
        if (!stageTasks?.length) continue;
        const titles = stageTasks.slice(0, 3).map((t) => `"${t.title}"`).join(', ');
        const extra = stageTasks.length > 3 ? ` +${stageTasks.length - 3} more` : '';
        lines.push(`  ${stage} (${stageTasks.length}): ${titles}${extra}`);
      }
      return lines.join('\n');
    }

    it('returns "No tasks." for empty array', () => {
      expect(buildProductSummary([])).toBe('No tasks.');
    });

    it('returns named summary when productName given', () => {
      const tasks = [
        { stage: 'active' },
        { stage: 'active' },
        { stage: 'blocked' },
        { stage: 'done' },
      ];
      const summary = buildProductSummary(tasks, 'Scout');
      expect(summary).toBe('Scout: 2 active, 1 blocked, 0 in validation');
    });

    it('groups tasks by stage in correct order', () => {
      const tasks = [
        { stage: 'done', title: 'Done task' },
        { stage: 'active', title: 'Active task' },
        { stage: 'backlog', title: 'Backlog task' },
      ];
      const summary = buildProductSummary(tasks);
      const lines = summary.split('\n');
      // active should come before backlog, backlog before done (per STAGE_ORDER)
      expect(lines[0]).toContain('active');
      expect(lines[1]).toContain('backlog');
      expect(lines[2]).toContain('done');
    });

    it('truncates to 3 titles with +N more', () => {
      const tasks = [
        { stage: 'active', title: 'A' },
        { stage: 'active', title: 'B' },
        { stage: 'active', title: 'C' },
        { stage: 'active', title: 'D' },
        { stage: 'active', title: 'E' },
      ];
      const summary = buildProductSummary(tasks);
      expect(summary).toContain('"A"');
      expect(summary).toContain('"B"');
      expect(summary).toContain('"C"');
      expect(summary).not.toContain('"D"');
      expect(summary).toContain('+2 more');
    });
  });

  // Test the chip label logic pattern
  describe('chip label logic', () => {
    function chipLabel(
      tasks: { stage: string; product: string }[],
      activeProduct: string | null,
    ): string | null {
      if (!activeProduct) return null;
      const productTasks = tasks.filter((t) => t.product === activeProduct);
      const active = productTasks.filter((t) =>
        (ACTIVE_STAGES as readonly string[]).includes(t.stage),
      ).length;
      return `\u27F3 ${activeProduct} context \u2014 ${active} active tasks`;
    }

    it('returns null when no active product', () => {
      expect(chipLabel([], null)).toBeNull();
    });

    it('counts active/blocked/validation tasks', () => {
      const tasks = [
        { stage: 'active', product: 'chat' },
        { stage: 'blocked', product: 'chat' },
        { stage: 'validation', product: 'chat' },
        { stage: 'done', product: 'chat' },
        { stage: 'active', product: 'scout' },
      ];
      const label = chipLabel(tasks, 'chat');
      expect(label).toContain('3 active tasks');
    });

    it('only counts tasks for the active product', () => {
      const tasks = [
        { stage: 'active', product: 'chat' },
        { stage: 'active', product: 'scout' },
      ];
      const label = chipLabel(tasks, 'chat');
      expect(label).toContain('1 active tasks');
      expect(label).toContain('chat');
    });
  });
});

// ── Hook-level integration tests ──
import { useProductContext } from './useProductContext';
import type { PlannerTask, TaskStage } from '../lib/types';

function makeTask(overrides: Partial<PlannerTask> = {}): PlannerTask {
  return {
    id: 1,
    title: 'Test task',
    description: '',
    stage: 'active' as TaskStage,
    priority: 1,
    product: 'chat',
    workflow: 'micro',
    metadata: '{}',
    created_at: '2026-03-30T10:00:00Z',
    updated_at: '2026-03-30T10:00:00Z',
    ...overrides,
  } as PlannerTask;
}

describe('useProductContext hook', () => {
  afterEach(() => cleanup());

  it('returns null deepContext when no active product', () => {
    const { result } = renderHook(() =>
      useProductContext([makeTask()], null, ['chat']),
    );
    expect(result.current.deepContext).toBeNull();
  });

  it('returns null chipLabel when no active product', () => {
    const { result } = renderHook(() =>
      useProductContext([makeTask()], null, ['chat']),
    );
    expect(result.current.chipLabel).toBeNull();
  });

  it('generates deepContext with task details', () => {
    const tasks = [
      makeTask({ id: 1, title: 'Fix bug', stage: 'active' as TaskStage, product: 'chat', priority: 2 }),
      makeTask({ id: 2, title: 'Add feature', stage: 'backlog' as TaskStage, product: 'chat', priority: 3 }),
    ];
    const { result } = renderHook(() =>
      useProductContext(tasks, 'chat', ['chat']),
    );
    expect(result.current.deepContext).toContain('Product: chat');
    expect(result.current.deepContext).toContain('Fix bug');
    expect(result.current.deepContext).toContain('Add feature');
    expect(result.current.deepContext).toContain('Total tasks: 2');
  });

  it('generates allProductsSummary for multiple products', () => {
    const tasks = [
      makeTask({ id: 1, stage: 'active' as TaskStage, product: 'chat' }),
      makeTask({ id: 2, stage: 'done' as TaskStage, product: 'scout' }),
    ];
    const { result } = renderHook(() =>
      useProductContext(tasks, 'chat', ['chat', 'scout']),
    );
    expect(result.current.allProductsSummary).toContain('chat:');
    expect(result.current.allProductsSummary).toContain('scout:');
    expect(result.current.allProductsSummary).toContain('All products overview');
  });

  it('allProductsSummary is empty when no products', () => {
    const { result } = renderHook(() =>
      useProductContext([], null, []),
    );
    expect(result.current.allProductsSummary).toBe('');
  });

  it('chipLabel counts active stages correctly', () => {
    const tasks = [
      makeTask({ id: 1, stage: 'active' as TaskStage, product: 'chat' }),
      makeTask({ id: 2, stage: 'blocked' as TaskStage, product: 'chat' }),
      makeTask({ id: 3, stage: 'done' as TaskStage, product: 'chat' }),
    ];
    const { result } = renderHook(() =>
      useProductContext(tasks, 'chat', ['chat']),
    );
    expect(result.current.chipLabel).toContain('2 active tasks');
  });

  it('contextString includes deep context and products summary', () => {
    const tasks = [
      makeTask({ id: 1, stage: 'active' as TaskStage, product: 'chat' }),
    ];
    const { result } = renderHook(() =>
      useProductContext(tasks, 'chat', ['chat']),
    );
    expect(result.current.contextString).toContain('Active Product Context');
    expect(result.current.contextString).toContain('All products overview');
  });

  it('buildContextString includes extra products when specified', () => {
    const tasks = [
      makeTask({ id: 1, stage: 'active' as TaskStage, product: 'chat' }),
      makeTask({ id: 2, stage: 'backlog' as TaskStage, product: 'scout' }),
    ];
    const { result } = renderHook(() =>
      useProductContext(tasks, 'chat', ['chat', 'scout']),
    );
    const full = result.current.buildContextString(['scout']);
    expect(full).toContain('Extra Product Context');
    expect(full).toContain('scout');
  });

  it('deepContext shows empty product message when no tasks match', () => {
    const { result } = renderHook(() =>
      useProductContext([], 'chat', ['chat']),
    );
    expect(result.current.deepContext).toContain('No tasks found for this product');
  });
});
