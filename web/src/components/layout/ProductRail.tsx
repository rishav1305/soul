import type { PlannerTask } from '../../lib/types.ts';

// Deterministic color from product name for unknown products
const PRODUCT_COLORS: Record<string, string> = {
  soul: 'text-soul bg-soul/10 border-soul',
  compliance: 'text-stage-validation bg-stage-validation/10 border-stage-validation',
  'compliance-go': 'text-stage-active bg-stage-active/10 border-stage-active',
  scout: 'text-stage-brainstorm bg-stage-brainstorm/10 border-stage-brainstorm',
};

const FALLBACK_COLORS = [
  'text-stage-active bg-stage-active/10 border-stage-active',
  'text-stage-brainstorm bg-stage-brainstorm/10 border-stage-brainstorm',
  'text-stage-validation bg-stage-validation/10 border-stage-validation',
  'text-stage-done bg-stage-done/10 border-stage-done',
  'text-stage-blocked bg-stage-blocked/10 border-stage-blocked',
  'text-soul bg-soul/10 border-soul',
];

function productColor(name: string): string {
  if (PRODUCT_COLORS[name]) return PRODUCT_COLORS[name];
  // Hash name to a fallback color
  let hash = 0;
  for (let i = 0; i < name.length; i++) hash = (hash * 31 + name.charCodeAt(i)) & 0xffff;
  return FALLBACK_COLORS[hash % FALLBACK_COLORS.length];
}

function productAbbr(name: string): string {
  const parts = name.replace(/[-_]/g, ' ').split(' ').filter(Boolean);
  if (parts.length === 1) return parts[0].slice(0, 2).toUpperCase();
  return (parts[0][0] + parts[1][0]).toUpperCase();
}

interface ProductRailProps {
  products: string[];
  activeProduct: string | null;
  tasks: PlannerTask[];
  onProductSelect: (product: string | null) => void;
  onSessionsToggle: () => void;
  onSettingsToggle: () => void;
  sessionsOpen: boolean;
  settingsOpen: boolean;
}

export default function ProductRail({
  products,
  activeProduct,
  tasks,
  onProductSelect,
  onSessionsToggle,
  onSettingsToggle,
  sessionsOpen,
  settingsOpen,
}: ProductRailProps) {
  // Count active tasks per product
  const activeCounts: Record<string, number> = {};
  for (const t of tasks) {
    if (t.product && (t.stage === 'active' || t.stage === 'blocked')) {
      activeCounts[t.product] = (activeCounts[t.product] ?? 0) + 1;
    }
  }

  return (
    <div className="w-14 h-full bg-surface border-r border-border-subtle flex flex-col items-center py-3 gap-1 shrink-0 z-10">
      {/* Soul logo — opens sessions drawer */}
      <button
        type="button"
        onClick={onSessionsToggle}
        className={`relative w-10 h-10 flex items-center justify-center rounded-lg transition-colors cursor-pointer mb-1 ${
          sessionsOpen ? 'bg-soul/15' : 'hover:bg-elevated'
        }`}
        title="Soul — Sessions"
      >
        <span className="absolute inset-0 -m-0.5 bg-soul/10 rounded-full blur-sm animate-soul-pulse pointer-events-none" />
        <span className="relative text-2xl text-soul leading-none">&#9670;</span>
      </button>

      <div className="w-7 border-t border-border-subtle my-1" />

      {/* Dynamic product list */}
      <div className="flex flex-col items-center gap-1 flex-1 w-full px-1 overflow-y-auto overflow-x-hidden">
        {products.map((product) => {
          const isActive = activeProduct === product;
          const colors = productColor(product);
          const abbr = productAbbr(product);
          const count = activeCounts[product] ?? 0;

          return (
            <button
              key={product}
              type="button"
              onClick={() => onProductSelect(isActive ? null : product)}
              title={product}
              className={`relative w-10 h-10 flex items-center justify-center rounded-lg text-xs font-display font-bold transition-all cursor-pointer border-l-2 ${
                isActive
                  ? `${colors} border-opacity-100`
                  : 'text-fg-muted hover:text-fg hover:bg-elevated border-transparent'
              }`}
            >
              {abbr}
              {count > 0 && (
                <span className="absolute -top-1 -right-1 w-4 h-4 bg-stage-active text-deep text-[9px] font-bold rounded-full flex items-center justify-center leading-none">
                  {count > 9 ? '9+' : count}
                </span>
              )}
            </button>
          );
        })}
      </div>

      <div className="w-7 border-t border-border-subtle my-1" />

      {/* All Tasks shortcut */}
      <button
        type="button"
        onClick={() => onProductSelect(null)}
        className={`w-10 h-10 flex items-center justify-center rounded-lg transition-colors cursor-pointer ${
          activeProduct === null ? 'bg-elevated text-fg' : 'text-fg-muted hover:text-fg hover:bg-elevated'
        }`}
        title="All Tasks"
      >
        <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M3 4l2 2 4-4" />
          <path d="M3 10l2 2 4-4" />
          <path d="M11 5h2M11 11h2" />
        </svg>
      </button>

      {/* Conversation History */}
      <button
        type="button"
        onClick={onSessionsToggle}
        className={`w-10 h-10 flex items-center justify-center rounded-lg transition-colors cursor-pointer ${
          sessionsOpen ? 'bg-elevated text-fg' : 'text-fg-muted hover:text-fg hover:bg-elevated'
        }`}
        title="Conversation History"
        data-testid="conversation-history-btn"
      >
        <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="8" cy="8" r="6" />
          <path d="M8 5v3.5l2.5 1.5" />
          <path d="M2.5 2.5 Q1 5 2 8" strokeWidth="1.3" />
          <path d="M2 5.5l0 2.5 2.5 0" strokeWidth="1.3" />
        </svg>
      </button>

      {/* Settings */}
      <button
        type="button"
        onClick={onSettingsToggle}
        className={`w-10 h-10 flex items-center justify-center rounded-lg transition-colors cursor-pointer ${
          settingsOpen ? 'bg-elevated text-fg' : 'text-fg-muted hover:text-fg hover:bg-elevated'
        }`}
        title="Settings"
      >
        <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <circle cx="8" cy="8" r="2" />
          <path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41" />
        </svg>
      </button>
    </div>
  );
}
