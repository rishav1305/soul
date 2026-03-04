import type { ChatPosition } from '../lib/types.ts';

interface SettingsPanelProps {
  open: boolean;
  onClose: () => void;
  chatPosition: ChatPosition;
  chatSplit: number;
  autoInjectContext: boolean;
  showContextChip: boolean;
  toastsEnabled: boolean;
  inlineBadgesEnabled: boolean;
  onChatPosition: (p: ChatPosition) => void;
  onChatSplit: (v: number) => void;
  onAutoInjectContext: (v: boolean) => void;
  onShowContextChip: (v: boolean) => void;
  onToastsEnabled: (v: boolean) => void;
  onInlineBadgesEnabled: (v: boolean) => void;
}

function Toggle({ checked, onChange }: { checked: boolean; onChange: (v: boolean) => void }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      onClick={() => onChange(!checked)}
      className={`relative w-10 h-5 rounded-full transition-colors cursor-pointer shrink-0 ${
        checked ? 'bg-soul' : 'bg-overlay border border-border-default'
      }`}
    >
      <span
        className={`absolute top-0.5 w-4 h-4 rounded-full bg-white shadow transition-transform ${
          checked ? 'translate-x-5' : 'translate-x-0.5'
        }`}
      />
    </button>
  );
}

export default function SettingsPanel({
  open,
  onClose,
  chatPosition,
  chatSplit,
  autoInjectContext,
  showContextChip,
  toastsEnabled,
  inlineBadgesEnabled,
  onChatPosition,
  onChatSplit,
  onAutoInjectContext,
  onShowContextChip,
  onToastsEnabled,
  onInlineBadgesEnabled,
}: SettingsPanelProps) {
  if (!open) return null;

  return (
    <>
      {/* Backdrop */}
      <div className="fixed inset-0 z-[800] bg-black/40" onClick={onClose} />

      {/* Slide-in panel */}
      <div className="fixed left-14 top-0 h-full w-80 bg-surface border-r border-border-subtle z-[801] flex flex-col animate-slide-left shadow-2xl">
        {/* Header */}
        <div className="glass flex items-center gap-2 h-14 px-4 shrink-0">
          <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" className="text-soul">
            <circle cx="8" cy="8" r="2" />
            <path d="M8 1v2M8 13v2M1 8h2M13 8h2M3.05 3.05l1.41 1.41M11.54 11.54l1.41 1.41M3.05 12.95l1.41-1.41M11.54 4.46l1.41-1.41" />
          </svg>
          <span className="font-display text-base font-semibold text-fg">Settings</span>
          <div className="flex-1" />
          <button
            type="button"
            onClick={onClose}
            className="w-8 h-8 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
              <path d="M4 4l8 8M12 4l-8 8" />
            </svg>
          </button>
        </div>

        <div className="flex-1 overflow-y-auto px-4 py-4 space-y-6">
          {/* Rail Position */}
          <section>
            <h3 className="text-xs font-display uppercase tracking-widest text-fg-muted mb-3">Rail Position</h3>
            <div className="flex gap-3">
              {(['bottom', 'top'] as ChatPosition[]).map((pos) => (
                <button
                  key={pos}
                  type="button"
                  onClick={() => onChatPosition(pos)}
                  className={`flex-1 py-2 rounded-lg text-sm font-display font-medium transition-colors cursor-pointer ${
                    chatPosition === pos
                      ? 'bg-soul/15 text-soul border border-soul/30'
                      : 'bg-elevated text-fg-muted hover:text-fg border border-border-default'
                  }`}
                >
                  {chatPosition === pos ? '●' : '○'} {pos.charAt(0).toUpperCase() + pos.slice(1)}
                </button>
              ))}
            </div>
          </section>

          {/* Chat/Tasks Split */}
          <section>
            <h3 className="text-xs font-display uppercase tracking-widest text-fg-muted mb-3">
              Chat / Tasks Split
              <span className="ml-2 text-fg-secondary normal-case text-[11px]">{chatSplit}% / {100 - chatSplit}%</span>
            </h3>
            <input
              type="range"
              min={10}
              max={90}
              value={chatSplit}
              onChange={(e) => onChatSplit(Number(e.target.value))}
              className="w-full accent-soul cursor-pointer"
            />
            <div className="flex justify-between text-[10px] text-fg-muted mt-1">
              <span>Chat 10%</span>
              <span>Chat 90%</span>
            </div>
          </section>

          {/* Context & Notifications */}
          <section>
            <h3 className="text-xs font-display uppercase tracking-widest text-fg-muted mb-3">Context & Notifications</h3>
            <div className="space-y-3">
              <label className="flex items-center justify-between gap-3 cursor-pointer">
                <div>
                  <p className="text-sm text-fg">Auto-inject context on new chat</p>
                  <p className="text-[11px] text-fg-muted">Prepend product context to first message</p>
                </div>
                <Toggle checked={autoInjectContext} onChange={onAutoInjectContext} />
              </label>

              <label className="flex items-center justify-between gap-3 cursor-pointer">
                <div>
                  <p className="text-sm text-fg">Show context chip on product switch</p>
                  <p className="text-[11px] text-fg-muted">Prompt to inject context in InputBar</p>
                </div>
                <Toggle checked={showContextChip} onChange={onShowContextChip} />
              </label>

              <label className="flex items-center justify-between gap-3 cursor-pointer">
                <div>
                  <p className="text-sm text-fg">Stage change toasts</p>
                  <p className="text-[11px] text-fg-muted">Show notifications when tasks move stages</p>
                </div>
                <Toggle checked={toastsEnabled} onChange={onToastsEnabled} />
              </label>

              <label className="flex items-center justify-between gap-3 cursor-pointer">
                <div>
                  <p className="text-sm text-fg">Inline task card badges</p>
                  <p className="text-[11px] text-fg-muted">Pulsing dot on cards after stage change</p>
                </div>
                <Toggle checked={inlineBadgesEnabled} onChange={onInlineBadgesEnabled} />
              </label>
            </div>
          </section>
        </div>
      </div>
    </>
  );
}
