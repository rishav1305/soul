import { useEffect } from 'react';
import { useObserve } from '../hooks/useObserve';
import { usePerformance } from '../hooks/usePerformance';
import { reportUsage } from '../lib/telemetry';
import type { ObservePillar, ObserveConstraint, ObserveOverview, ObserveTailResponse, ObserveTab, ObserveProduct } from '../lib/types';

// --- Status colors ---
const constraintStatusColor: Record<string, string> = {
  pass: 'border-emerald-400',
  warn: 'border-amber-400',
  fail: 'border-red-400',
  static: 'border-zinc-500',
};

const constraintStatusBg: Record<string, string> = {
  pass: 'bg-emerald-400/10 text-emerald-400',
  warn: 'bg-amber-400/10 text-amber-400',
  fail: 'bg-red-400/10 text-red-400',
  static: 'bg-zinc-500/10 text-fg-muted',
};

function pillarHealthColor(pillar: ObservePillar): string {
  if (pillar.fail > 0) return 'text-red-400';
  if (pillar.warn > 0) return 'text-amber-400';
  return 'text-emerald-400';
}

function pillarHealthBorder(pillar: ObservePillar): string {
  if (pillar.fail > 0) return 'border-red-400/30';
  if (pillar.warn > 0) return 'border-amber-400/30';
  return 'border-emerald-400/30';
}

// --- StatCard ---
function StatCard({ label, value, sub }: { label: string; value: string | number; sub?: string }) {
  return (
    <div className="bg-surface rounded-lg p-4" data-testid={`stat-${label.toLowerCase().replace(/\s+/g, '-')}`}>
      <div className="text-xs text-fg-muted mb-1">{label}</div>
      <div className="text-2xl font-bold text-fg">{value}</div>
      {sub && <div className="text-xs text-fg-muted mt-1">{sub}</div>}
    </div>
  );
}

// --- PillarStrip ---
function PillarStrip({ pillars, activeTab, onTabClick }: { pillars: ObservePillar[]; activeTab: ObserveTab; onTabClick: (tab: ObserveTab) => void }) {
  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-2" data-testid="pillar-strip">
      {pillars.map(pillar => {
        const tabName = pillar.name.toLowerCase() as ObserveTab;
        const isActive = activeTab === tabName;
        return (
          <button
            key={pillar.name}
            onClick={() => onTabClick(tabName)}
            className={`bg-surface rounded-lg p-3 text-left border transition-colors ${
              isActive ? 'border-soul' : pillarHealthBorder(pillar)
            } hover:bg-elevated`}
            data-testid={`pillar-card-${pillar.name.toLowerCase()}`}
          >
            <div className="text-xs text-fg-muted capitalize mb-1">{pillar.name}</div>
            <div className={`text-lg font-bold ${pillarHealthColor(pillar)}`}>
              {pillar.pass}/{pillar.total}
            </div>
            <div className="text-[10px] text-fg-muted mt-0.5">
              {pillar.fail > 0 && <span className="text-red-400 mr-1">{pillar.fail}F</span>}
              {pillar.warn > 0 && <span className="text-amber-400 mr-1">{pillar.warn}W</span>}
              {pillar.static > 0 && <span className="text-fg-muted">{pillar.static}S</span>}
            </div>
          </button>
        );
      })}
    </div>
  );
}

// --- ConstraintRow ---
function ConstraintRow({ constraint }: { constraint: ObserveConstraint }) {
  return (
    <div
      className={`bg-surface rounded-lg p-3 border-l-2 ${constraintStatusColor[constraint.status]}`}
      data-testid={`constraint-${constraint.name}`}
    >
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0 flex-1">
          <div className="text-sm font-medium text-fg truncate">{constraint.name}</div>
          <div className="text-xs text-fg-muted mt-0.5">{constraint.target}</div>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          {constraint.value && (
            <span className="text-xs text-fg-secondary font-mono">{constraint.value}</span>
          )}
          <span className={`px-2 py-0.5 text-[10px] rounded-full uppercase font-medium ${constraintStatusBg[constraint.status]}`}>
            {constraint.status}
          </span>
        </div>
      </div>
      <div className="text-[10px] text-fg-muted mt-1">{constraint.enforcement}</div>
    </div>
  );
}

// --- OverviewTab ---
function OverviewTab({ overview }: { overview: ObserveOverview }) {
  const upHours = Math.floor(overview.status.uptime_seconds / 3600);
  const upMins = Math.floor((overview.status.uptime_seconds % 3600) / 60);

  return (
    <div className="space-y-4" data-testid="observe-overview">
      {/* Status stats */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
        <StatCard label="Uptime" value={`${upHours}h ${upMins}m`} />
        <StatCard label="Sessions" value={overview.status.sessions} />
        <StatCard label="Messages" value={overview.status.messages} />
        <StatCard label="Active Streams" value={overview.status.active_streams} />
        <StatCard label="Total Events" value={overview.status.total_events} />
        <StatCard label="Errors" value={overview.status.errors} />
      </div>

      {/* Cost card */}
      <div className="bg-surface rounded-lg p-4" data-testid="cost-card">
        <h3 className="text-sm font-medium text-fg-muted mb-3">Cost</h3>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
          <div>
            <div className="text-xs text-fg-muted">Input Tokens</div>
            <div className="text-lg font-bold text-fg">{overview.cost.input_tokens.toLocaleString()}</div>
          </div>
          <div>
            <div className="text-xs text-fg-muted">Output Tokens</div>
            <div className="text-lg font-bold text-fg">{overview.cost.output_tokens.toLocaleString()}</div>
          </div>
          <div>
            <div className="text-xs text-fg-muted">Requests</div>
            <div className="text-lg font-bold text-fg">{overview.cost.requests}</div>
          </div>
          <div>
            <div className="text-xs text-fg-muted">Estimated USD</div>
            <div className="text-lg font-bold text-soul">${overview.cost.estimated_usd.toFixed(4)}</div>
          </div>
        </div>
      </div>

      {/* Alerts card */}
      <div className="bg-surface rounded-lg p-4" data-testid="alerts-card">
        <h3 className="text-sm font-medium text-fg-muted mb-3">
          Alerts {overview.alerts.breaches && overview.alerts.breaches.length > 0 && (
            <span className="text-red-400 ml-1">({overview.alerts.breaches.length})</span>
          )}
        </h3>
        {(!overview.alerts.breaches || overview.alerts.breaches.length === 0) ? (
          <div className="text-sm text-fg-muted">No active alerts.</div>
        ) : (
          <div className="space-y-2">
            {overview.alerts.breaches.map((alert, i) => (
              <div key={i} className="bg-red-400/5 border border-red-400/20 rounded-lg p-3 text-sm">
                <div className="flex items-center justify-between">
                  <span className="text-fg font-medium">{alert.metric}.{alert.field}</span>
                  <span className="text-[10px] text-fg-muted">{new Date(alert.timestamp).toLocaleTimeString()}</span>
                </div>
                <div className="text-xs text-fg-muted mt-1">
                  Value: <span className="text-red-400 font-mono">{alert.value}</span> / Threshold: <span className="text-fg-secondary font-mono">{alert.threshold}</span>
                  <span className="ml-2 text-amber-400 uppercase text-[10px]">{alert.severity}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

// --- PillarTab ---
function PillarTab({ pillar }: { pillar: ObservePillar }) {
  return (
    <div className="space-y-3" data-testid={`pillar-tab-${pillar.name.toLowerCase()}`}>
      <div className="flex items-center justify-between">
        <div>
          <h3 className="text-sm font-medium text-fg capitalize">{pillar.name}</h3>
          <p className="text-xs text-fg-muted mt-0.5">{pillar.description}</p>
        </div>
        <div className={`text-lg font-bold ${pillarHealthColor(pillar)}`}>
          {pillar.pass}/{pillar.total}
        </div>
      </div>
      <div className="space-y-2">
        {pillar.constraints.map(c => (
          <ConstraintRow key={c.name} constraint={c} />
        ))}
      </div>
      {pillar.constraints.length === 0 && (
        <div className="text-sm text-fg-muted py-4">No constraints defined.</div>
      )}
    </div>
  );
}

// --- TailTab ---
function TailTab({ tail }: { tail: ObserveTailResponse }) {
  return (
    <div className="space-y-2" data-testid="observe-tail">
      <div className="text-xs text-fg-muted mb-2">{tail.total} total events</div>
      <div className="space-y-1 max-h-[calc(100vh-320px)] overflow-y-auto">
        {tail.events.map((event, i) => (
          <div key={i} className="bg-surface rounded-lg px-3 py-2 text-sm font-mono" data-testid={`tail-event-${i}`}>
            <div className="flex items-center gap-3">
              <span className="text-fg-muted text-xs shrink-0">
                {new Date(event.timestamp).toLocaleTimeString()}
              </span>
              <span className="text-soul text-xs font-medium">{event.event}</span>
            </div>
            {Object.keys(event.data).length > 0 && (
              <div className="text-[11px] text-fg-muted mt-1 truncate">
                {JSON.stringify(event.data)}
              </div>
            )}
          </div>
        ))}
        {tail.events.length === 0 && (
          <div className="text-sm text-fg-muted py-4">No events recorded.</div>
        )}
      </div>
    </div>
  );
}

// --- Main Page ---
export function ObservePage() {
  usePerformance('ObservePage');
  const { pillars, overview, tail, loading, error, activeTab, setActiveTab, product, setProduct, refresh } = useObserve();

  useEffect(() => { reportUsage('page.view', { page: 'observe' }); }, []);

  const tabs: ObserveTab[] = ['overview', 'performant', 'robust', 'resilient', 'secure', 'sovereign', 'transparent', 'tail'];
  const products: { value: ObserveProduct; label: string }[] = [
    { value: '', label: 'All Products' },
    { value: 'chat', label: 'Chat' },
    { value: 'tasks', label: 'Tasks' },
    { value: 'tutor', label: 'Tutor' },
    { value: 'projects', label: 'Projects' },
  ];

  // Find the active pillar for pillar tabs
  const activePillar = pillars.find(p => p.name.toLowerCase() === activeTab);

  return (
    <div data-testid="observe-page" className="h-full overflow-y-auto p-4 sm:p-6 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-fg">Observe</h2>
        <div className="flex items-center gap-2">
          <select
            value={product}
            onChange={(e) => setProduct(e.target.value as ObserveProduct)}
            className="soul-select text-xs"
            data-testid="observe-product-filter"
          >
            {products.map(p => (
              <option key={p.value} value={p.value}>{p.label}</option>
            ))}
          </select>
          <button onClick={refresh} className="px-3 py-1 text-xs rounded bg-elevated hover:bg-overlay text-fg-secondary transition-colors" data-testid="observe-refresh">
            Refresh
          </button>
        </div>
      </div>

      {/* Pillar health strip */}
      {pillars.length > 0 && (
        <PillarStrip pillars={pillars} activeTab={activeTab} onTabClick={setActiveTab} />
      )}

      {/* Tab nav */}
      <nav className="tab-scroll flex gap-1 border-b border-border-subtle pb-px" data-testid="observe-tabs">
        {tabs.map(tab => (
          <button key={tab} onClick={() => setActiveTab(tab)}
            className={`px-3 py-1.5 text-sm rounded-t transition-colors capitalize whitespace-nowrap ${activeTab === tab ? 'bg-surface text-fg' : 'text-fg-muted hover:text-zinc-200'}`}
            data-testid={`tab-${tab}`}>
            {tab}
          </button>
        ))}
      </nav>

      {error && <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" data-testid="observe-error">{error}</div>}

      {/* Tab content */}
      {activeTab === 'overview' && overview && <OverviewTab overview={overview} />}
      {activePillar && ['performant', 'robust', 'resilient', 'secure', 'sovereign', 'transparent'].includes(activeTab) && (
        <PillarTab pillar={activePillar} />
      )}
      {activeTab === 'tail' && tail && <TailTab tail={tail} />}
      {loading && !overview && pillars.length === 0 && !tail && (
        <div className="text-center py-8 text-fg-muted">Loading...</div>
      )}
    </div>
  );
}
