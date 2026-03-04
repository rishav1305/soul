import { useMemo } from 'react';
import type { PlannerTask, TaskStage } from '../lib/types.ts';

const ACTIVE_STAGES: TaskStage[] = ['active', 'blocked', 'validation'];

function buildProductSummary(tasks: PlannerTask[], product: string): string {
  const productTasks = tasks.filter((t) => t.product === product);
  const active = productTasks.filter((t) => t.stage === 'active').length;
  const blocked = productTasks.filter((t) => t.stage === 'blocked').length;
  const validation = productTasks.filter((t) => t.stage === 'validation').length;
  return `${product}: ${active} active, ${blocked} blocked, ${validation} in validation`;
}

function buildDeepContext(tasks: PlannerTask[], product: string): string {
  const productTasks = tasks.filter((t) => t.product === product);
  const byStage = {} as Record<TaskStage, PlannerTask[]>;
  const stages: TaskStage[] = ['backlog', 'brainstorm', 'active', 'blocked', 'validation', 'done'];
  for (const s of stages) {
    byStage[s] = productTasks.filter((t) => t.stage === s);
  }

  const lines: string[] = [`=== ${product} Context ===`];
  for (const stage of stages) {
    const stageTasks = byStage[stage];
    if (stageTasks.length === 0) continue;
    lines.push(`\n[${stage.toUpperCase()}] (${stageTasks.length})`);
    for (const t of stageTasks) {
      const blockerNote = t.blocker ? ` — BLOCKED: ${t.blocker}` : '';
      lines.push(`  #${t.id} ${t.title}${blockerNote}`);
    }
  }

  return lines.join('\n');
}

export function useProductContext(
  tasks: PlannerTask[],
  activeProduct: string | null,
) {
  const products = useMemo(() => {
    const set = new Set<string>();
    for (const t of tasks) {
      if (t.product) set.add(t.product);
    }
    return Array.from(set).sort();
  }, [tasks]);

  /** Full context string — deep for active product, summary for others */
  const contextString = useMemo(() => {
    const parts: string[] = [];

    if (activeProduct) {
      parts.push(buildDeepContext(tasks, activeProduct));
      parts.push('');
      parts.push('=== Other Products (summary) ===');
      for (const p of products) {
        if (p !== activeProduct) {
          parts.push(buildProductSummary(tasks, p));
        }
      }
    } else {
      parts.push('=== All Products Summary ===');
      for (const p of products) {
        parts.push(buildProductSummary(tasks, p));
      }
    }

    return parts.join('\n');
  }, [tasks, activeProduct, products]);

  /** Light summary for the chip label */
  const chipLabel = useMemo(() => {
    if (!activeProduct) return null;
    const productTasks = tasks.filter((t) => t.product === activeProduct);
    const active = productTasks.filter((t) => ACTIVE_STAGES.includes(t.stage)).length;
    return `⟳ ${activeProduct} context — ${active} active tasks`;
  }, [tasks, activeProduct]);

  return { contextString, chipLabel };
}
