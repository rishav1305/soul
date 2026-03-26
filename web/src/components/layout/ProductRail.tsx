import { useState, useRef, useEffect } from 'react';
import type { PlannerTask, PanelPosition, DrawerLayout, ProductInfo } from '../../lib/types.ts';

function productAbbr(name: string): string {
  const parts = name.replace(/[-_]/g, ' ').split(' ').filter(Boolean);
  if (parts.length === 1) return (parts[0] ?? '').slice(0, 2).toUpperCase();
  return ((parts[0]?.[0] ?? '') + (parts[1]?.[0] ?? '')).toUpperCase();
}

// ── Toggle component (reused from SettingsPanel) ──
function Toggle({ checked, onChange, label, description }: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
  description?: string;
}) {
  return (
    <button
      type="button"
      onClick={() => onChange(!checked)}
      className="flex items-start gap-3 w-full text-left cursor-pointer group"
    >
      <div className={`relative shrink-0 mt-0.5 w-8 rounded-full transition-colors ${checked ? 'bg-soul' : 'bg-overlay'}`}
        style={{ height: '18px', width: '32px' }}
      >
        <span
          className={`absolute top-0.5 w-3.5 h-3.5 rounded-full bg-white shadow transition-transform ${checked ? 'translate-x-[14px]' : 'translate-x-0.5'}`}
        />
      </div>
      <div>
        <div className={`text-sm font-display transition-colors ${checked ? 'text-fg' : 'text-fg-secondary'}`}>
          {label}
        </div>
        {description && (
          <div className="text-[10px] text-fg-muted mt-0.5">{description}</div>
        )}
      </div>
    </button>
  );
}

// ── Auth section ──
function AuthSection() {
  const [status, setStatus] = useState<'idle' | 'loading' | 'ok' | 'error'>('idle');
  const [message, setMessage] = useState('');

  const handleReauth = async () => {
    setStatus('loading');
    try {
      const res = await fetch('/api/reauth', { method: 'POST' });
      const data = await res.json();
      if (res.ok) {
        setStatus('ok');
        setMessage(data.message || 'Credentials refreshed');
      } else {
        setStatus('error');
        setMessage(data.error || 'Failed to refresh');
      }
    } catch {
      setStatus('error');
      setMessage('Network error');
    }
    setTimeout(() => setStatus('idle'), 4000);
  };

  return (
    <section>
      <h3 className="text-[11px] font-display font-semibold uppercase tracking-widest text-fg-muted mb-3">
        AI Authentication
      </h3>
      <button
        type="button"
        onClick={handleReauth}
        disabled={status === 'loading'}
        className="w-full flex items-center justify-center gap-2 px-3 py-2 rounded-lg border border-border-subtle text-sm font-display text-fg-secondary hover:border-border-default hover:text-fg transition-colors cursor-pointer disabled:opacity-50"
      >
        <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
          <path d="M1 7a6 6 0 0111.196-3M13 7A6 6 0 011.804 10" />
          <path d="M1 1v3h3M13 13v-3h-3" />
        </svg>
        {status === 'loading' ? 'Refreshing...' : 'Refresh OAuth'}
      </button>
      {status === 'ok' && <div className="mt-2 text-[10px] text-green-400 font-mono">{message}</div>}
      {status === 'error' && <div className="mt-2 text-[10px] text-red-400 font-mono">{message}</div>}
    </section>
  );
}

// ── Gear icon SVG ──
const GearIcon = ({ size = 18 }: { size?: number }) => (
  <svg width={size} height={size} viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round">
    <circle cx="8" cy="8" r="2.5" />
    <path d="M6.8 1.5h2.4l.3 1.7a5.2 5.2 0 0 1 1.3.7l1.6-.6.9 1.5-1.3 1.1a5 5 0 0 1 0 1.5l1.3 1.1-.9 1.5-1.6-.6a5.2 5.2 0 0 1-1.3.7l-.3 1.7H6.8l-.3-1.7a5.2 5.2 0 0 1-1.3-.7l-1.6.6-.9-1.5 1.3-1.1a5 5 0 0 1 0-1.5L2.7 4.8l.9-1.5 1.6.6a5.2 5.2 0 0 1 1.3-.7z" />
  </svg>
);

// ── Panel toggle icon ──
const PanelIcon = ({ expanded }: { expanded: boolean }) => (
  <svg width="18" height="18" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round">
    <rect x="1.5" y="2" width="13" height="12" rx="1.5" />
    <path d="M5.5 2v12" />
    {expanded ? (
      <path d="M9 8l-2 0M9 6l-2 2 2 2" />
    ) : (
      <path d="M8 8l2 0M8 6l2 2-2 2" />
    )}
  </svg>
);

// ── Settings content (inline in panel) ──
interface SettingsContentProps {
  onBack: () => void;
  chatPosition: PanelPosition;
  setChatPosition: (pos: PanelPosition) => void;
  tasksPosition: PanelPosition;
  setTasksPosition: (pos: PanelPosition) => void;
  drawerLayout: DrawerLayout;
  setDrawerLayout: (v: DrawerLayout) => void;
  autoInjectContext: boolean;
  setAutoInjectContext: (v: boolean) => void;
  showContextChip: boolean;
  setShowContextChip: (v: boolean) => void;
  toastsEnabled: boolean;
  setToastsEnabled: (v: boolean) => void;
  inlineBadgesEnabled: boolean;
  setInlineBadgesEnabled: (v: boolean) => void;
}

function PositionSwitch({ value, onChange, label }: {
  value: PanelPosition;
  onChange: (pos: PanelPosition) => void;
  label: string;
}) {
  const options: PanelPosition[] = ['top', 'bottom', 'right'];
  return (
    <div className="space-y-1.5">
      <div className="text-xs font-display text-fg-secondary">{label}</div>
      <div className="flex rounded-lg border border-border-subtle overflow-hidden">
        {options.map((pos) => (
          <button
            key={pos}
            type="button"
            onClick={() => onChange(pos)}
            className={`flex-1 px-2 py-1.5 text-xs font-display capitalize transition-colors cursor-pointer ${
              value === pos
                ? 'bg-soul/15 text-soul'
                : 'text-fg-secondary hover:text-fg hover:bg-elevated'
            }`}
          >
            {pos === 'right' ? 'Right' : pos === 'top' ? 'Top' : 'Bottom'}
          </button>
        ))}
      </div>
    </div>
  );
}

function SettingsContent({
  onBack,
  chatPosition,
  setChatPosition,
  tasksPosition,
  setTasksPosition,
  drawerLayout,
  setDrawerLayout,
  autoInjectContext,
  setAutoInjectContext,
  showContextChip,
  setShowContextChip,
  toastsEnabled,
  setToastsEnabled,
  inlineBadgesEnabled,
  setInlineBadgesEnabled,
}: SettingsContentProps) {
  return (
    <div className="flex flex-col h-full">
      {/* Header */}
      <div className="flex items-center gap-2 px-4 h-12 border-b border-border-subtle shrink-0">
        <button
          type="button"
          onClick={onBack}
          className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
        >
          <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
            <path d="M9 2L3 7l6 5" />
          </svg>
        </button>
        <span className="font-display text-sm font-semibold text-fg">Settings</span>
      </div>

      <div className="flex-1 overflow-y-auto px-4 py-4 space-y-6">
        {/* Panel Positions */}
        <section>
          <h3 className="text-[11px] font-display font-semibold uppercase tracking-widest text-fg-muted mb-3">
            Panel Positions
          </h3>
          <div className="space-y-3">
            <PositionSwitch value={chatPosition} onChange={setChatPosition} label="Chat" />
            <PositionSwitch value={tasksPosition} onChange={setTasksPosition} label="Tasks" />
          </div>
        </section>

        {/* Drawer Layout */}
        <section>
          <h3 className="text-[11px] font-display font-semibold uppercase tracking-widest text-fg-muted mb-3">
            Drawer Layout
          </h3>
          {(() => {
            const mixed = chatPosition !== tasksPosition && chatPosition !== 'right' && tasksPosition !== 'right';
            const effectiveLayout = mixed ? 'independent' : drawerLayout;
            return (
              <div className={`flex rounded-lg border border-border-subtle overflow-hidden ${mixed ? 'opacity-50' : ''}`}>
                {(['split', 'independent'] as DrawerLayout[]).map((mode) => (
                  <button
                    key={mode}
                    type="button"
                    onClick={() => !mixed && setDrawerLayout(mode)}
                    className={`flex-1 px-3 py-2 text-sm font-display capitalize transition-colors ${mixed ? 'cursor-not-allowed' : 'cursor-pointer'} ${
                      effectiveLayout === mode
                        ? 'bg-soul/15 text-soul border-soul'
                        : 'text-fg-secondary hover:text-fg hover:bg-elevated'
                    }`}
                  >
                    {mode === 'split' ? 'Split' : 'Independent'}
                  </button>
                ))}
              </div>
            );
          })()}
          {chatPosition !== tasksPosition && chatPosition !== 'right' && tasksPosition !== 'right' && (
            <div className="text-[10px] text-fg-muted mt-1.5">Forced to Independent when positions differ</div>
          )}
        </section>

        {/* Context Injection */}
        <section>
          <h3 className="text-[11px] font-display font-semibold uppercase tracking-widest text-fg-muted mb-3">
            Context Injection
          </h3>
          <div className="space-y-4">
            <Toggle checked={autoInjectContext} onChange={setAutoInjectContext} label="Auto-inject on new chat" description="Sends active product context when starting a new session" />
            <Toggle checked={showContextChip} onChange={setShowContextChip} label="Show chip on product switch" description="Offers inject prompt when navigating to a different product" />
          </div>
        </section>

        {/* Notifications */}
        <section>
          <h3 className="text-[11px] font-display font-semibold uppercase tracking-widest text-fg-muted mb-3">
            Notifications
          </h3>
          <div className="space-y-4">
            <Toggle checked={toastsEnabled} onChange={setToastsEnabled} label="Stage change toasts" description="Pop-up notifications when a task moves between stages" />
            <Toggle checked={inlineBadgesEnabled} onChange={setInlineBadgesEnabled} label="Inline task card badges" description="Pulsing dot on the task card after a stage change" />
          </div>
        </section>

        {/* AI Authentication */}
        <AuthSection />

        {/* Version info */}
        <section className="pt-2">
          <div className="text-[10px] text-fg-muted font-mono">Soul v0.2.0-alpha</div>
        </section>
      </div>
    </div>
  );
}

// ── Main component ──

interface ProductRailProps {
  products: string[];
  activeProduct: string | null;
  tasks: PlannerTask[];
  productMetadata?: Map<string, ProductInfo>;
  onProductSelect: (product: string | null) => void;
  expanded: boolean;
  onToggleExpanded: () => void;
  settingsOpen: boolean;
  onSettingsToggle: () => void;
  // Settings props (passed through)
  chatPosition: PanelPosition;
  setChatPosition: (pos: PanelPosition) => void;
  tasksPosition: PanelPosition;
  setTasksPosition: (pos: PanelPosition) => void;
  drawerLayout: DrawerLayout;
  setDrawerLayout: (v: DrawerLayout) => void;
  autoInjectContext: boolean;
  setAutoInjectContext: (v: boolean) => void;
  showContextChip: boolean;
  setShowContextChip: (v: boolean) => void;
  toastsEnabled: boolean;
  setToastsEnabled: (v: boolean) => void;
  inlineBadgesEnabled: boolean;
  setInlineBadgesEnabled: (v: boolean) => void;
}

const RAIL_WIDTH = 56;   // w-14
const PANEL_WIDTH = 220;

export { RAIL_WIDTH, PANEL_WIDTH };

export default function ProductRail({
  products,
  activeProduct,
  tasks,
  productMetadata,
  onProductSelect,
  expanded,
  onToggleExpanded,
  settingsOpen,
  onSettingsToggle,
  chatPosition,
  setChatPosition,
  tasksPosition,
  setTasksPosition,
  drawerLayout,
  setDrawerLayout,
  autoInjectContext,
  setAutoInjectContext,
  showContextChip,
  setShowContextChip,
  toastsEnabled,
  setToastsEnabled,
  inlineBadgesEnabled,
  setInlineBadgesEnabled,
}: ProductRailProps) {
  // Scroll active product into view
  const activeRef = useRef<HTMLButtonElement | null>(null);
  useEffect(() => {
    activeRef.current?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  }, [activeProduct]);

  // Count active tasks per product
  const activeCounts: Record<string, number> = {};
  for (const t of tasks) {
    if (t.product && (t.stage === 'active' || t.stage === 'blocked')) {
      activeCounts[t.product] = (activeCounts[t.product] ?? 0) + 1;
    }
  }

  const width = expanded ? PANEL_WIDTH : RAIL_WIDTH;

  // ── Settings view (overlays entire panel in expanded mode) ──
  if (settingsOpen && expanded) {
    return (
      <div
        data-testid="product-rail"
        className="fixed left-0 top-0 h-screen bg-surface border-r border-border-subtle z-20 transition-[width] duration-200"
        style={{ width }}
      >
        <SettingsContent
          onBack={onSettingsToggle}
          chatPosition={chatPosition}
          setChatPosition={setChatPosition}
          tasksPosition={tasksPosition}
          setTasksPosition={setTasksPosition}
          drawerLayout={drawerLayout}
          setDrawerLayout={setDrawerLayout}
          autoInjectContext={autoInjectContext}
          setAutoInjectContext={setAutoInjectContext}
          showContextChip={showContextChip}
          setShowContextChip={setShowContextChip}
          toastsEnabled={toastsEnabled}
          setToastsEnabled={setToastsEnabled}
          inlineBadgesEnabled={inlineBadgesEnabled}
          setInlineBadgesEnabled={setInlineBadgesEnabled}
        />
      </div>
    );
  }

  return (
    <div
      data-testid="product-rail"
      className="fixed left-0 top-0 h-screen bg-surface border-r border-border-subtle flex flex-col items-center py-3 gap-1 z-20 transition-[width] duration-200"
      style={{ width }}
    >
      {/* Soul logo */}
      <div className={`flex items-center gap-2 shrink-0 mb-1 ${expanded ? 'w-full px-4' : ''}`}>
        <div className="relative w-11 h-11 flex items-center justify-center shrink-0">
          <span className="absolute inset-0 -m-1 bg-soul/15 rounded-full blur-md animate-soul-pulse pointer-events-none" />
          <span className="relative text-3xl text-soul leading-none drop-shadow-[0_0_8px_var(--color-soul)]">&#9670;</span>
        </div>
        {expanded && (
          <span className="font-display text-lg font-bold text-fg tracking-tight">Soul</span>
        )}
      </div>

      <div className={`border-t border-border-subtle my-1 ${expanded ? 'w-full mx-4' : 'w-7'}`} />

      {/* Product list */}
      <div className={`flex flex-col gap-1 flex-1 w-full overflow-y-auto overflow-x-hidden ${expanded ? 'px-2' : 'px-1 items-center'}`}>
        {products.map((product) => {
          const isActive = activeProduct === product;
          const meta = productMetadata?.get(product);
          const displayLabel = meta?.label || product;
          const abbr = productAbbr(displayLabel);
          const count = activeCounts[product] ?? 0;

          if (expanded) {
            return (
              <button
                key={product}
                ref={isActive ? activeRef : null}
                type="button"
                onClick={() => onProductSelect(product)}
                title={displayLabel}
                className={`relative flex items-center gap-3 px-3 py-2 rounded-lg text-xs font-display font-bold transition-all cursor-pointer border-l-2 ${
                  isActive
                    ? 'text-soul bg-soul/10 border-soul'
                    : 'text-fg-secondary hover:text-fg hover:bg-elevated border-transparent'
                }`}
              >
                <span className="w-8 h-8 flex items-center justify-center rounded shrink-0 text-xs">
                  {abbr}
                </span>
                <span className="text-sm font-display font-semibold truncate">{displayLabel}</span>
                {count > 0 && (
                  <span className="ml-auto w-5 h-5 bg-stage-active text-deep text-[10px] font-bold rounded-full flex items-center justify-center leading-none shrink-0">
                    {count > 9 ? '9+' : count}
                  </span>
                )}
              </button>
            );
          }

          return (
            <button
              key={product}
              ref={isActive ? activeRef : null}
              type="button"
              onClick={() => onProductSelect(product)}
              title={displayLabel}
              className={`relative w-10 h-10 flex items-center justify-center rounded-lg text-xs font-display font-bold transition-all cursor-pointer border-l-2 ${
                isActive
                  ? 'text-soul bg-soul/10 border-soul'
                  : 'text-fg-secondary hover:text-fg hover:bg-elevated border-transparent'
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

      <div className={`border-t border-border-subtle my-1 ${expanded ? 'w-full mx-4' : 'w-7'}`} />

      {/* Panel toggle */}
      <button
        type="button"
        onClick={onToggleExpanded}
        className={`flex items-center gap-2 rounded-lg transition-colors cursor-pointer text-fg-secondary hover:text-fg hover:bg-elevated ${
          expanded ? 'w-full mx-2 px-3 py-2' : 'w-10 h-10 justify-center'
        }`}
        title={expanded ? 'Collapse panel' : 'Expand panel'}
      >
        <PanelIcon expanded={expanded} />
        {expanded && <span className="text-sm font-display">Collapse</span>}
      </button>

      {/* Settings */}
      <button
        type="button"
        onClick={() => {
          if (!expanded) {
            onToggleExpanded();
          }
          onSettingsToggle();
        }}
        className={`flex items-center gap-2 rounded-lg transition-colors cursor-pointer ${
          settingsOpen ? 'bg-elevated text-fg' : 'text-fg-secondary hover:text-fg hover:bg-elevated'
        } ${expanded ? 'w-full mx-2 px-3 py-2' : 'w-10 h-10 justify-center'}`}
        title="Settings"
      >
        <GearIcon />
        {expanded && <span className="text-sm font-display">Settings</span>}
      </button>
    </div>
  );
}
