import { useEffect } from 'react';
import { useSentinel } from '../hooks/useSentinel';
import { usePerformance } from '../hooks/usePerformance';
import { reportUsage } from '../lib/telemetry';
import { ChallengeList } from '../components/sentinel/ChallengeList';
import { ChallengeSession } from '../components/sentinel/ChallengeSession';
import { SandboxConfig } from '../components/sentinel/SandboxConfig';
import { SandboxChat } from '../components/sentinel/SandboxChat';
import { ProgressBoard } from '../components/sentinel/ProgressBoard';
import { ScanResults } from '../components/sentinel/ScanResults';

export function SentinelPage() {
  usePerformance('SentinelPage');
  const {
    challenges,
    progress,
    activeChallenge,
    activeChallengeId,
    attackHistory,
    sandboxConfig,
    sandboxMessages,
    scanResults,
    loading,
    error,
    activeTab,
    setActiveTab,
    startChallenge,
    submitFlag,
    attack,
    configureSandbox,
    scanProduct,
    sendSandboxMessage,
    requestHint,
    exitChallenge,
    refresh,
  } = useSentinel();

  useEffect(() => { reportUsage('page.view', { page: 'sentinel' }); }, []);

  const tabs = ['challenges', 'sandbox', 'progress'] as const;

  return (
    <div data-testid="sentinel-page" className="h-full overflow-y-auto p-4 sm:p-6 space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-fg">Sentinel</h2>
        <button
          onClick={refresh}
          className="px-3 py-1 text-xs rounded bg-elevated hover:bg-overlay text-fg-secondary transition-colors"
          data-testid="sentinel-refresh"
        >
          Refresh
        </button>
      </div>

      {/* Tab nav */}
      <nav className="tab-scroll flex gap-1 border-b border-border-subtle pb-px" data-testid="sentinel-tabs" role="tablist" aria-label="Sentinel sections">
        {tabs.map(tab => (
          <button
            key={tab}
            onClick={() => setActiveTab(tab)}
            role="tab"
            aria-selected={activeTab === tab}
            aria-controls={`sentinel-panel-${tab}`}
            className={`px-3 py-1.5 text-sm rounded-t transition-colors capitalize whitespace-nowrap ${
              activeTab === tab ? 'bg-surface text-fg' : 'text-fg-muted hover:text-zinc-200'
            }`}
            data-testid={`tab-${tab}`}
          >
            {tab}
          </button>
        ))}
      </nav>

      {error && (
        <div className="text-red-400 text-sm bg-red-400/10 px-3 py-2 rounded" role="alert" data-testid="sentinel-error">
          {error}
        </div>
      )}

      {/* Tab content */}
      {activeTab === 'challenges' && (
        activeChallenge && activeChallengeId !== null ? (
          <ChallengeSession
            challenge={activeChallenge}
            challengeId={activeChallengeId}
            challengeMeta={challenges.find(c => c.id === activeChallengeId) ?? null}
            attackHistory={attackHistory}
            onAttack={attack}
            onSubmitFlag={submitFlag}
            onRequestHint={requestHint}
            onExit={exitChallenge}
          />
        ) : (
          <ChallengeList challenges={challenges} onStart={startChallenge} />
        )
      )}

      {activeTab === 'sandbox' && (
        <div className="space-y-4">
          <SandboxConfig config={sandboxConfig} onSave={configureSandbox} />
          <SandboxChat config={sandboxConfig} messages={sandboxMessages} onSend={sendSandboxMessage} />
          <ScanResults results={scanResults} onScan={scanProduct} />
        </div>
      )}

      {activeTab === 'progress' && progress && (
        <ProgressBoard progress={progress} />
      )}

      {loading && !activeChallenge && challenges.length === 0 && !progress && (
        <div className="text-center py-8 text-fg-muted" role="status" aria-live="polite">Loading...</div>
      )}
    </div>
  );
}
