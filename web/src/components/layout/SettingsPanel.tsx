import type { HorizontalRailPosition } from '../../lib/types.ts';

interface SettingsPanelProps {
  onClose: () => void;
  railPosition: HorizontalRailPosition;
  setRailPosition: (pos: HorizontalRailPosition) => void;
  chatSplitPct: number;
  setChatSplitPct: (pct: number) => void;
  autoInjectContext: boolean;
  setAutoInjectContext: (v: boolean) => void;
  showContextChip: boolean;
  setShowContextChip: (v: boolean) => void;
  toastsEnabled: boolean;
  setToastsEnabled: (v: boolean) => void;
  inlineBadgesEnabled: boolean;
  setInlineBadgesEnabled: (v: boolean) => void;
}

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
      {/* Toggle pill */}
      <div className={`relative shrink-0 mt-0.5 w-8 h-4.5 rounded-full transition-colors ${checked ? 'bg-soul' : 'bg-overlay'}`}
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

export default function SettingsPanel({
  onClose,
  railPosition,
  setRailPosition,
  chatSplitPct,
  setChatSplitPct,
  autoInjectContext,
  setAutoInjectContext,
  showContextChip,
  setShowContextChip,
  toastsEnabled,
  setToastsEnabled,
  inlineBadgesEnabled,
  setInlineBadgesEnabled,
}: SettingsPanelProps) {
  return (
    <div className="absolute inset-0 z-50 flex">
      {/* Backdrop */}
      <div className="absolute inset-0 bg-black/40" onClick={onClose} />

      {/* Panel — slides in from left rail */}
      <div className="relative z-10 w-72 h-full bg-surface border-r border-border-default flex flex-col shadow-2xl animate-slide-left ml-14">
        {/* Header */}
        <div className="flex items-center gap-2 px-4 h-12 border-b border-border-subtle shrink-0">
          <span className="font-display text-sm font-semibold text-fg">Settings</span>
          <div className="flex-1" />
          <button
            type="button"
            onClick={onClose}
            className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
          >
            <svg width="14" height="14" viewBox="0 0 14 14" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round">
              <path d="M1 1l12 12M13 1L1 13" />
            </svg>
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-4 py-4 space-y-6">
          {/* Rail Position */}
          <section>
            <h3 className="text-[11px] font-display font-semibold uppercase tracking-widest text-fg-muted mb-3">
              Chat &amp; Tasks Rail Position
            </h3>
            <div className="flex flex-col gap-2">
              {(['bottom', 'top'] as HorizontalRailPosition[]).map((pos) => (
                <button
                  key={pos}
                  type="button"
                  onClick={() => setRailPosition(pos)}
                  className={`flex items-center gap-3 px-3 py-2.5 rounded-lg border transition-colors cursor-pointer text-left ${
                    railPosition === pos
                      ? 'border-soul bg-soul/10 text-soul'
                      : 'border-border-subtle text-fg-secondary hover:border-border-default hover:text-fg'
                  }`}
                >
                  <span className={`w-4 h-4 rounded-full border-2 flex items-center justify-center shrink-0 ${
                    railPosition === pos ? 'border-soul' : 'border-fg-muted'
                  }`}>
                    {railPosition === pos && <span className="w-2 h-2 rounded-full bg-soul" />}
                  </span>
                  <span className="text-sm font-display capitalize">{pos}</span>
                </button>
              ))}
            </div>
          </section>

          {/* Chat / Tasks Split */}
          <section>
            <h3 className="text-[11px] font-display font-semibold uppercase tracking-widest text-fg-muted mb-3">
              Chat / Tasks Split
            </h3>
            <div className="space-y-2">
              <input
                type="range"
                min={30}
                max={80}
                step={5}
                value={chatSplitPct}
                onChange={(e) => setChatSplitPct(Number(e.target.value))}
                className="w-full accent-soul cursor-pointer"
              />
              <div className="flex justify-between text-[10px] text-fg-muted font-mono">
                <span>Chat {chatSplitPct}%</span>
                <span>Tasks {100 - chatSplitPct}%</span>
              </div>
            </div>
          </section>

          {/* Context Injection */}
          <section>
            <h3 className="text-[11px] font-display font-semibold uppercase tracking-widest text-fg-muted mb-3">
              Context Injection
            </h3>
            <div className="space-y-4">
              <Toggle
                checked={autoInjectContext}
                onChange={setAutoInjectContext}
                label="Auto-inject on new chat"
                description="Automatically sends active product context when starting a new session"
              />
              <Toggle
                checked={showContextChip}
                onChange={setShowContextChip}
                label="Show chip on product switch"
                description="Offers a one-click inject prompt when you navigate to a different product mid-chat"
              />
            </div>
          </section>

          {/* Notifications */}
          <section>
            <h3 className="text-[11px] font-display font-semibold uppercase tracking-widest text-fg-muted mb-3">
              Notifications
            </h3>
            <div className="space-y-4">
              <Toggle
                checked={toastsEnabled}
                onChange={setToastsEnabled}
                label="Stage change toasts"
                description="Pop-up notifications (top-right) when a task moves between stages"
              />
              <Toggle
                checked={inlineBadgesEnabled}
                onChange={setInlineBadgesEnabled}
                label="Inline task card badges"
                description="Pulsing dot on the task card for 90s after a stage change"
              />
            </div>
          </section>

          {/* Version info */}
          <section className="pt-2">
            <div className="text-[10px] text-fg-muted font-mono space-y-1">
              <div>Soul v0.2.0-alpha</div>
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
