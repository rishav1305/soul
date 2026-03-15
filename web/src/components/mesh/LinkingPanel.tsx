import { useState } from 'react';

interface LinkingPanelProps {
  linkCode: string | null;
  onGenerateCode: () => Promise<void>;
  onLinkNode: (code: string) => Promise<void>;
}

export function LinkingPanel({ linkCode, onGenerateCode, onLinkNode }: LinkingPanelProps) {
  const [enterCode, setEnterCode] = useState('');
  const [generating, setGenerating] = useState(false);
  const [joining, setJoining] = useState(false);
  const [statusMessage, setStatusMessage] = useState<string | null>(null);

  const handleGenerate = async () => {
    setGenerating(true);
    setStatusMessage(null);
    try {
      await onGenerateCode();
      setStatusMessage('Pairing code generated.');
    } catch {
      setStatusMessage('Failed to generate code.');
    } finally {
      setGenerating(false);
    }
  };

  const handleJoin = async () => {
    if (!enterCode.trim()) return;
    setJoining(true);
    setStatusMessage(null);
    try {
      await onLinkNode(enterCode.trim());
      setStatusMessage('Node linked successfully.');
      setEnterCode('');
    } catch {
      setStatusMessage('Failed to link node.');
    } finally {
      setJoining(false);
    }
  };

  return (
    <div className="bg-surface rounded-lg p-4 space-y-4" data-testid="linking-panel">
      {/* Generate code */}
      <div className="space-y-2">
        <h4 className="text-sm font-medium text-fg-muted">Generate Pairing Code</h4>
        <button
          onClick={handleGenerate}
          disabled={generating}
          className="px-4 py-2 text-sm rounded bg-soul/20 text-soul hover:bg-soul/30 transition-colors disabled:opacity-50"
          data-testid="generate-code"
        >
          {generating ? 'Generating...' : 'Generate Code'}
        </button>
        {linkCode && (
          <div className="mt-2 px-3 py-2 bg-elevated rounded font-mono text-sm text-fg" data-testid="pairing-code">
            {linkCode}
          </div>
        )}
      </div>

      {/* Enter code to join */}
      <div className="space-y-2">
        <h4 className="text-sm font-medium text-fg-muted">Join with Code</h4>
        <div className="flex gap-2">
          <input
            type="text"
            value={enterCode}
            onChange={e => setEnterCode(e.target.value)}
            placeholder="Enter pairing code"
            className="flex-1 px-3 py-2 text-sm bg-elevated border border-border-default rounded text-fg placeholder:text-fg-muted focus:outline-none focus:border-soul"
            data-testid="enter-code"
          />
          <button
            onClick={handleJoin}
            disabled={joining || !enterCode.trim()}
            className="px-4 py-2 text-sm rounded bg-emerald-500/20 text-emerald-400 hover:bg-emerald-500/30 transition-colors disabled:opacity-50"
            data-testid="join-btn"
          >
            {joining ? 'Joining...' : 'Join'}
          </button>
        </div>
      </div>

      {/* Status message */}
      {statusMessage && (
        <div className="text-sm text-fg-secondary px-3 py-2 bg-elevated rounded" data-testid="link-status">
          {statusMessage}
        </div>
      )}
    </div>
  );
}
