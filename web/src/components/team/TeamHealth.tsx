import { useTeamHealth } from '../../hooks/useTeamHealth.ts';

/**
 * TeamHealth — service status, token usage, restart history, system resources, alert feed.
 * Route: /team/health
 */
export default function TeamHealth() {
  const { health, loading, error, refresh } = useTeamHealth();

  if (loading && !health) {
    return (
      <div data-testid="team-health-loading" className="flex items-center justify-center h-full text-sm text-fg-muted">
        Loading health data...
      </div>
    );
  }

  if (error && !health) {
    return (
      <div data-testid="team-health-error" className="flex items-center justify-center h-full">
        <div className="text-center space-y-2">
          <p className="text-sm text-red-400">Failed to load health data</p>
          <button
            type="button"
            data-testid="team-health-retry"
            onClick={refresh}
            className="px-3 py-1.5 text-xs bg-soul text-deep rounded hover:bg-soul/85 transition-colors cursor-pointer"
          >
            Retry
          </button>
        </div>
      </div>
    );
  }

  if (!health) return null;

  const maxTokens = Math.max(...health.tokenUsage.map((u) => u.tokensUsed), 1);

  return (
    <div data-testid="team-health" className="flex flex-col h-full bg-deep overflow-hidden">
      {/* ── Header ── */}
      <header className="flex items-center justify-between px-6 py-3 border-b border-border-subtle shrink-0">
        <h1 className="text-sm font-semibold text-fg">Team Health</h1>
        <button
          type="button"
          data-testid="team-health-refresh"
          onClick={refresh}
          className="w-7 h-7 flex items-center justify-center rounded text-fg-muted hover:text-fg hover:bg-overlay/60 transition-colors cursor-pointer"
          aria-label="Refresh health data"
          title="Refresh"
        >
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
            <path d="M13.5 8A5.5 5.5 0 1 1 8 2.5c1.5 0 2.9.6 3.9 1.6L14 6" />
            <path d="M14 2v4h-4" />
          </svg>
        </button>
      </header>

      <div className="flex-1 overflow-y-auto p-6 space-y-6">
        {/* ── Service status ── */}
        <section aria-labelledby="health-services-heading">
          <h2 id="health-services-heading" className="text-[11px] text-fg-muted font-medium uppercase tracking-wider mb-3">
            Services
          </h2>
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3">
            {health.services.map((svc) => (
              <div
                key={svc.name}
                data-testid={`service-status-${svc.name.toLowerCase()}`}
                className="flex items-center justify-between bg-surface border border-border-subtle rounded-lg px-4 py-3"
              >
                <div className="flex items-center gap-2">
                  <span
                    className={`w-2.5 h-2.5 rounded-full shrink-0 ${
                      svc.status === 'up' ? 'bg-green-500' :
                      svc.status === 'degraded' ? 'bg-yellow-400' :
                      'bg-red-500'
                    }`}
                    aria-label={`${svc.name} is ${svc.status}`}
                    role="img"
                  />
                  <span className="text-sm font-medium text-fg">{svc.name}</span>
                </div>
                <div className="text-right">
                  <div className={`text-xs font-medium capitalize ${
                    svc.status === 'up' ? 'text-green-400' :
                    svc.status === 'degraded' ? 'text-yellow-400' :
                    'text-red-400'
                  }`}>
                    {svc.status}
                  </div>
                  {svc.latencyMs !== undefined && (
                    <div className="text-[10px] text-fg-muted">{svc.latencyMs}ms</div>
                  )}
                </div>
              </div>
            ))}
          </div>
        </section>

        {/* ── Token usage ── */}
        <section aria-labelledby="health-tokens-heading">
          <h2 id="health-tokens-heading" className="text-[11px] text-fg-muted font-medium uppercase tracking-wider mb-3">
            Token Usage
          </h2>
          <div className="bg-surface border border-border-subtle rounded-xl p-4 space-y-3">
            {health.tokenUsage.map((u) => {
              const pct = Math.round((u.tokensUsed / u.tokensMax) * 100);
              const barWidth = Math.round((u.tokensUsed / maxTokens) * 100);
              return (
                <div key={u.agent} data-testid={`token-bar-${u.agent}`} className="flex items-center gap-3">
                  <span className="text-xs text-fg-secondary w-16 shrink-0">{u.agent}</span>
                  {/* SVG bar */}
                  <svg
                    className="flex-1 h-4"
                    aria-label={`${u.agent} token usage: ${pct}%`}
                    role="img"
                  >
                    <rect x="0" y="4" width="100%" height="8" rx="4" fill="var(--color-overlay)" />
                    <rect
                      x="0" y="4"
                      width={`${barWidth}%`}
                      height="8"
                      rx="4"
                      fill={pct >= 80 ? '#ef4444' : pct >= 60 ? '#eab308' : '#22c55e'}
                    />
                  </svg>
                  <span className="text-[10px] text-fg-muted w-20 text-right shrink-0">
                    {(u.tokensUsed / 1000).toFixed(0)}k / {(u.tokensMax / 1000).toFixed(0)}k
                  </span>
                  <span className="text-[10px] text-fg-muted w-10 text-right shrink-0">
                    ${u.costUsd.toFixed(2)}
                  </span>
                </div>
              );
            })}
          </div>
        </section>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
          {/* ── System resources ── */}
          <section aria-labelledby="health-resources-heading">
            <h2 id="health-resources-heading" className="text-[11px] text-fg-muted font-medium uppercase tracking-wider mb-3">
              System Resources{health.resources.hostname ? ` (${health.resources.hostname})` : ''}
            </h2>
            <div className="bg-surface border border-border-subtle rounded-xl p-4 space-y-4">
              {/* CPU temp */}
              <div>
                <div className="flex items-center justify-between mb-1">
                  <span className="text-xs text-fg-secondary">CPU Temp</span>
                  <span className={`text-xs font-medium ${health.resources.cpuTempC >= 65 ? 'text-red-400' : health.resources.cpuTempC >= 55 ? 'text-yellow-400' : 'text-green-400'}`}>
                    {health.resources.cpuTempC}°C
                  </span>
                </div>
                <div
                  className="h-1.5 bg-overlay rounded-full overflow-hidden"
                  role="progressbar"
                  aria-valuenow={health.resources.cpuTempC}
                  aria-valuemin={0}
                  aria-valuemax={80}
                  aria-label={`CPU temperature: ${health.resources.cpuTempC}°C`}
                >
                  <div
                    data-testid="cpu-temp-bar"
                    className={`h-full rounded-full ${health.resources.cpuTempC >= 65 ? 'bg-red-500' : health.resources.cpuTempC >= 55 ? 'bg-yellow-500' : 'bg-green-500'}`}
                    style={{ width: `${(health.resources.cpuTempC / 80) * 100}%` }}
                  />
                </div>
              </div>

              {/* Memory */}
              <div>
                <div className="flex items-center justify-between mb-1">
                  <span className="text-xs text-fg-secondary">Memory</span>
                  <span className="text-xs font-medium text-fg-secondary">
                    {health.resources.memoryTotalMb > 0
                      ? `${(health.resources.memoryUsedMb / 1024).toFixed(1)} / ${(health.resources.memoryTotalMb / 1024).toFixed(1)} GB`
                      : 'N/A'}
                  </span>
                </div>
                <div
                  className="h-1.5 bg-overlay rounded-full overflow-hidden"
                  role="progressbar"
                  aria-valuenow={health.resources.memoryTotalMb > 0 ? health.resources.memoryUsedMb : 0}
                  aria-valuemin={0}
                  aria-valuemax={health.resources.memoryTotalMb > 0 ? health.resources.memoryTotalMb : 1}
                  aria-label={
                    health.resources.memoryTotalMb > 0
                      ? `Memory: ${(health.resources.memoryUsedMb / 1024).toFixed(1)}GB used`
                      : 'Memory: data unavailable'
                  }
                >
                  <div
                    data-testid="memory-bar"
                    className="h-full rounded-full bg-blue-500"
                    style={{
                      width: health.resources.memoryTotalMb > 0
                        ? `${(health.resources.memoryUsedMb / health.resources.memoryTotalMb) * 100}%`
                        : '0%',
                    }}
                  />
                </div>
              </div>

              {/* Load avg */}
              <div className="flex items-center justify-between text-xs">
                <span className="text-fg-secondary">Load avg</span>
                <span className="text-fg-muted font-mono">
                  {health.resources.loadAvg1.toFixed(1)} / {health.resources.loadAvg5.toFixed(1)} / {health.resources.loadAvg15.toFixed(1)}
                </span>
              </div>
            </div>
          </section>

          {/* ── Restart history ── */}
          <section aria-labelledby="health-restarts-heading">
            <h2 id="health-restarts-heading" className="text-[11px] text-fg-muted font-medium uppercase tracking-wider mb-3">
              Restart History
            </h2>
            <div
              data-testid="restart-history"
              className="bg-surface border border-border-subtle rounded-xl divide-y divide-border-subtle overflow-hidden"
            >
              {health.restartHistory.slice(0, 10).map((ev) => {
                const d = new Date(ev.timestamp);
                const label = d.toLocaleDateString([], { month: 'short', day: 'numeric' })
                  + ' ' + d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
                return (
                  <div key={ev.id} data-testid={`restart-event-${ev.id}`} className="flex items-start gap-3 px-4 py-2.5">
                    <div className="w-1.5 h-1.5 rounded-full bg-fg-muted mt-1.5 shrink-0" aria-hidden="true" />
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center gap-1.5">
                        <span className="text-xs font-medium text-fg-secondary">{ev.agent}</span>
                        <time dateTime={d.toISOString()} className="text-[10px] text-fg-muted">{label}</time>
                      </div>
                      <p className="text-[11px] text-fg-muted truncate">{ev.reason}</p>
                    </div>
                  </div>
                );
              })}
            </div>
          </section>
        </div>

        {/* ── Alert feed ── */}
        <section aria-labelledby="health-alerts-heading">
          <h2 id="health-alerts-heading" className="text-[11px] text-fg-muted font-medium uppercase tracking-wider mb-3">
            Alerts
          </h2>
          <div
            data-testid="alert-feed"
            className="bg-surface border border-border-subtle rounded-xl divide-y divide-border-subtle overflow-hidden"
            role="list"
            aria-label="Recent health alerts"
          >
            {health.alerts.map((alert) => {
              const d = new Date(alert.timestamp);
              const label = d.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
              return (
                <div
                  key={alert.id}
                  data-testid={`alert-${alert.id}`}
                  role="listitem"
                  className="flex items-start gap-3 px-4 py-3"
                >
                  <span
                    className={`text-[10px] font-bold uppercase px-1.5 py-0.5 rounded shrink-0 mt-0.5 ${
                      alert.level === 'error' ? 'bg-red-900/40 text-red-400' :
                      alert.level === 'warn' ? 'bg-yellow-900/40 text-yellow-400' :
                      'bg-zinc-800 text-zinc-400'
                    }`}
                    aria-label={`${alert.level} alert`}
                  >
                    {alert.level}
                  </span>
                  <div className="flex-1 min-w-0">
                    <p className="text-sm text-fg-secondary leading-snug">{alert.message}</p>
                    {alert.agent && (
                      <span className="text-[10px] text-fg-muted">Agent: {alert.agent}</span>
                    )}
                  </div>
                  <time dateTime={d.toISOString()} className="text-[10px] text-fg-muted shrink-0">
                    {label}
                  </time>
                </div>
              );
            })}
          </div>
        </section>
      </div>
    </div>
  );
}
