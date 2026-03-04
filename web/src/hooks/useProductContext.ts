import { useMemo } from 'react';
import type { PlannerTask, TaskStage } from '../lib/types.ts';

const STAGE_ORDER: TaskStage[] = ['active', 'blocked', 'validation', 'brainstorm', 'backlog', 'done'];

function buildProductSummary(productTasks: PlannerTask[]): string {
  if (productTasks.length === 0) return 'No tasks.';
  const byStage: Partial<Record<TaskStage, PlannerTask[]>> = {};
  for (const t of productTasks) {
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

function buildDeepContext(productName: string, productTasks: PlannerTask[]): string {
  if (productTasks.length === 0) {
    return `Product: ${productName}\nNo tasks found for this product.`;
  }
  const byStage: Partial<Record<TaskStage, PlannerTask[]>> = {};
  for (const t of productTasks) {
    if (!byStage[t.stage]) byStage[t.stage] = [];
    byStage[t.stage]!.push(t);
  }
  const lines: string[] = [`Product: ${productName}`, `Total tasks: ${productTasks.length}`];
  for (const stage of STAGE_ORDER) {
    const stageTasks = byStage[stage];
    if (!stageTasks?.length) continue;
    lines.push(`\n[${stage.toUpperCase()}] — ${stageTasks.length} task(s)`);
    for (const t of stageTasks) {
      lines.push(`  #${t.id} (P${t.priority}) ${t.title}`);
      if (t.description) lines.push(`    ${t.description.slice(0, 120)}`);
      if (t.blocker) lines.push(`    ⚠ Blocked: ${t.blocker}`);
      if (t.substep) lines.push(`    → Substep: ${t.substep}`);
    }
  }
  return lines.join('\n');
}

export function useProductContext(
  tasks: PlannerTask[],
  activeProduct: string | null,
  products: string[],
) {
  const deepContext = useMemo(() => {
    if (!activeProduct) return null;
    const productTasks = tasks.filter((t) => t.product === activeProduct);
    return buildDeepContext(activeProduct, productTasks);
  }, [tasks, activeProduct]);

  const allProductsSummary = useMemo(() => {
    if (products.length === 0) return '';
    const lines: string[] = ['All products overview:'];
    for (const p of products) {
      const ptasks = tasks.filter((t) => t.product === p);
      const active = ptasks.filter((t) => t.stage === 'active').length;
      const blocked = ptasks.filter((t) => t.stage === 'blocked').length;
      const done = ptasks.filter((t) => t.stage === 'done').length;
      lines.push(`  ${p}: ${ptasks.length} total, ${active} active, ${blocked} blocked, ${done} done`);
    }
    return lines.join('\n');
  }, [tasks, products]);

  const buildContextString = useMemo(() => {
    return (includeProducts?: string[]): string => {
      const parts: string[] = [];
      if (deepContext && activeProduct) {
        parts.push(`=== Active Product Context ===\n${deepContext}`);
      }
      if (allProductsSummary) {
        parts.push(`=== ${allProductsSummary}`);
      }
      if (includeProducts?.length) {
        const extra: string[] = [];
        for (const p of includeProducts) {
          if (p === activeProduct) continue;
          const ptasks = tasks.filter((t) => t.product === p);
          extra.push(`${p}:\n${buildProductSummary(ptasks)}`);
        }
        if (extra.length) parts.push(`=== Extra Product Context ===\n${extra.join('\n\n')}`);
      }
      return parts.join('\n\n');
    };
  }, [deepContext, allProductsSummary, activeProduct, tasks]);

  return { deepContext, allProductsSummary, buildContextString };
}
