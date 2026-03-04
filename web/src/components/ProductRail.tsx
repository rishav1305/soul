import { useMemo } from 'react';
import type { PlannerTask, TaskStage } from '../lib/types.ts';

// Deterministic color from product name hash
const STAGE_COLORS = [
  '#b07ce8', // brainstorm purple
  '#4ca8e8', // active blue
  '#4ce88c', // done green
  '#e8a849', // validation gold
  '#e85c5c', // blocked red
  '#7c7c8a', // backlog grey
];

function hashColor(name: string): string {
  let h = 0;
  for (let i = 0; i < name.length; i++) {
    h = (h * 31 + name.charCodeAt(i)) >>> 0;
  }
  return STAGE_COLORS[h % STAGE_COLORS.length];
}

function productAbbrev(name: string): string {
  const words = name.replace(/-/g, ' ').split(/\s+/);
  if (words.length >= 2) return (words[0][0] + words[1][0]).toUpperCase();
  return name.slice(0, 2).toUpperCase();
}

function taskCountForProduct(tasks: PlannerTask[], product: string): number {
  const active: TaskStage[] = ['active', 'blocked', 'validation'];
  return tasks.filter((t) => t.product === product && active.includes(t.stage)).length;
}

interface ProductRailProps {
  tasks: PlannerTask[];
  activeProduct: string | null;
  onProductSelect: (product: string | null) => void;
  onLogoClick: () => void;
  onSettingsClick: () => void;
}

export default function ProductRail({
  tasks,
  activeProduct,
  onProductSelect,
  onLogoClick,
  onSettingsClick,
}: ProductRailProps) {
  // Derive unique products from task.product field
  const products = useMemo(() => {
    const set = new Set<string>();
    for (const t of tasks) {
      if (t.product) set.add(t.product);
    }
    return Array.from(set).sort();
  }, [tasks]);

  return (
    <div className="w-14 h-full bg-surface border-r border-border-subtle flex flex-col items-center py-3 gap-1 shrink-0">
      {/* Soul logo — opens sessions overlay */}
      <button
        type="button"
        onClick={onLogoClick}
        className="w-10 h-10 flex items-center justify-center rounded-lg hover:bg-elevated transition-colors cursor-pointer mb-2"
        title="Soul — Sessions"
      >
        <span className="relative">
          <span className="absolute inset-0 -m-1 bg-soul/10 rounded-full blur-sm animate-soul-pulse" />
          <span className="relative text-2xl text-soul leading-none">&#9670;</span>
        </span>
      </button>

      <div className="w-6 border-t border-border-subtle mb-1" />

      {/* Dynamic product list */}
      <div className="flex-1 flex flex-col items-center gap-1 overflow-y-auto w-full px-2 overflow-x-hidden">
        {products.map((product) => {
          const isActive = activeProduct === product;
          const abbrev = productAbbrev(product);
          const color = hashColor(product);
          const count = taskCountForProduct(tasks, product);

          return (
            <div key={product} className="relative w-full flex justify-center">
              <button
                type="button"
                onClick={() => onProductSelect(isActive ? null : product)}
                title={product}
                className={`relative w-10 h-10 flex items-center justify-center rounded-lg text-xs font-display font-bold transition-all cursor-pointer ${
                  isActive
                    ? 'border-l-2 border-soul bg-soul/10 text-soul'
                    : 'text-fg-muted hover:bg-elevated hover:text-fg'
                }`}
                style={!isActive ? { color } : undefined}
              >
                {abbrev}
                {count > 0 && (
                  <span
                    className="absolute -top-1 -right-1 min-w-[16px] h-4 px-0.5 rounded-full text-[9px] font-bold flex items-center justify-center text-deep"
                    style={{ backgroundColor: color }}
                  >
                    {count}
                  </span>
                )}
              </button>
            </div>
          );
        })}
      </div>

      <div className="w-6 border-t border-border-subtle mt-1" />

      {/* All tasks shortcut */}
      <button
        type="button"
        onClick={() => onProductSelect(null)}
        title="All tasks"
        className={`w-10 h-10 flex items-center justify-center rounded-lg transition-colors cursor-pointer ${
          activeProduct === null
            ? 'bg-soul/10 text-soul'
            : 'text-fg-muted hover:bg-elevated hover:text-fg'
        }`}
      >
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M3 4l2 2 4-4" />
          <path d="M3 10l2 2 4-4" />
          <path d="M11 5h3M11 11h3" />
        </svg>
      </button>

      {/* Settings */}
      <button
        type="button"
        onClick={onSettingsClick}
        title="Settings"
        className="w-10 h-10 flex items-center justify-center rounded-lg text-fg-muted hover:bg-elevated hover:text-fg transition-colors cursor-pointer"
      >
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="8" cy="8" r="2" />
          <path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41" />
        </svg>
      </button>
    </div>
  );
}
