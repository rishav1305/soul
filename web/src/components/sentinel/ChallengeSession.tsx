import { useState, useRef, useEffect } from 'react';
import type { AttackEntry, ChallengeSession as ChallengeSessionType, Challenge, FlagResult } from '../../hooks/useSentinel';

interface ChallengeSessionProps {
  challenge: ChallengeSessionType;
  challengeId: string;
  challengeMeta: Challenge | null;
  attackHistory: AttackEntry[];
  onAttack: (payload: string, challengeId?: string) => Promise<void>;
  onSubmitFlag: (id: string, flag: string) => Promise<FlagResult | null>;
  onRequestHint: (challengeId: string) => Promise<string | null>;
  onExit: () => void;
}

export function ChallengeSession({
  challenge,
  challengeId,
  challengeMeta,
  attackHistory,
  onAttack,
  onSubmitFlag,
  onRequestHint,
  onExit,
}: ChallengeSessionProps) {
  const [attackInput, setAttackInput] = useState('');
  const [flagInput, setFlagInput] = useState('');
  const [sending, setSending] = useState(false);
  const [flagMessage, setFlagMessage] = useState<string | null>(null);
  const [flagCorrect, setFlagCorrect] = useState<boolean | null>(null);
  const [hints, setHints] = useState<string[]>([]);
  const [hintLoading, setHintLoading] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  const turnCount = challenge.turn_count;
  const maxTurns = challengeMeta?.max_turns ?? 10;
  const totalHints = challengeMeta?.hints?.length ?? 0;
  const objective = challengeMeta?.objective ?? '';
  const description = challengeMeta?.description ?? '';

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [attackHistory]);

  const handleSendAttack = async () => {
    if (!attackInput.trim() || sending) return;
    const payload = attackInput.trim();
    setAttackInput('');
    setSending(true);
    try {
      await onAttack(payload, challengeId);
    } finally {
      setSending(false);
    }
  };

  const handleSubmitFlag = async () => {
    if (!flagInput.trim()) return;
    setFlagMessage(null);
    setFlagCorrect(null);
    const result = await onSubmitFlag(challengeId, flagInput.trim());
    if (result) {
      setFlagMessage(result.message);
      setFlagCorrect(result.correct);
      if (result.correct) {
        setFlagInput('');
      }
    }
  };

  const handleRequestHint = async () => {
    if (hintLoading) return;
    setHintLoading(true);
    const hint = await onRequestHint(challengeId);
    if (hint) {
      setHints(prev => [...prev, hint]);
    }
    setHintLoading(false);
  };

  const handleAttackKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSendAttack();
    }
  };

  const handleFlagKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleSubmitFlag();
    }
  };

  return (
    <div className="flex flex-col h-full space-y-4" data-testid="challenge-session">
      {/* Header */}
      <div className="bg-surface rounded-lg p-4 space-y-2">
        <div className="flex items-center justify-between">
          <h3 className="text-sm font-medium text-fg">Active Challenge</h3>
          <div className="flex items-center gap-3">
            <span className="text-xs text-fg-muted" data-testid="turn-counter">
              Turn {turnCount}/{maxTurns}
            </span>
            <button
              onClick={onExit}
              className="px-2 py-1 text-xs rounded bg-elevated hover:bg-overlay text-fg-secondary transition-colors"
              data-testid="exit-challenge"
            >
              Exit
            </button>
          </div>
        </div>
        <p className="text-xs text-fg-muted">{description}</p>
        {objective && (
          <div className="bg-elevated rounded px-3 py-2">
            <span className="text-xs text-soul font-medium">Objective: </span>
            <span className="text-xs text-fg">{objective}</span>
          </div>
        )}
      </div>

      {/* Hints */}
      {hints.length > 0 && (
        <div className="space-y-1">
          {hints.map((hint, i) => (
            <div key={i} className="bg-amber-500/10 border border-amber-500/20 rounded px-3 py-2 text-xs text-amber-300">
              Hint {i + 1}: {hint}
            </div>
          ))}
        </div>
      )}

      {/* Attack history */}
      <div className="flex-1 overflow-y-auto space-y-2 min-h-0 bg-surface rounded-lg p-3" data-testid="attack-history">
        {attackHistory.length === 0 && (
          <div className="text-xs text-fg-muted text-center py-8">Send your first attack payload to begin.</div>
        )}
        {attackHistory.map(entry => (
          <div
            key={entry.id}
            className={`rounded-lg px-3 py-2 text-sm ${
              entry.role === 'attacker'
                ? 'bg-elevated ml-8 text-fg'
                : 'bg-red-500/10 border border-red-500/20 mr-8 text-fg'
            }`}
          >
            <div className="text-[10px] text-fg-muted mb-1">
              {entry.role === 'attacker' ? 'You' : 'Defender'}
            </div>
            <div className="whitespace-pre-wrap break-words">{entry.content}</div>
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>

      {/* Attack input */}
      <div className="flex gap-2">
        <div className="flex-1 flex gap-2">
          <input
            type="text"
            value={attackInput}
            onChange={e => setAttackInput(e.target.value)}
            onKeyDown={handleAttackKeyDown}
            placeholder="Enter attack payload..."
            className="flex-1 bg-elevated border border-border-default rounded-lg px-3 py-2 text-sm text-fg placeholder:text-fg-muted focus:outline-none focus:border-soul"
            data-testid="attack-input"
            disabled={sending || turnCount >= maxTurns}
          />
          <button
            onClick={handleSendAttack}
            disabled={sending || !attackInput.trim() || turnCount >= maxTurns}
            className="px-4 py-2 text-sm rounded-lg bg-soul text-deep font-medium hover:bg-soul/85 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            data-testid="send-attack"
          >
            Send
          </button>
        </div>
        <button
          onClick={handleRequestHint}
          disabled={hintLoading || hints.length >= totalHints}
          className="px-3 py-2 text-xs rounded-lg bg-amber-500/20 text-amber-400 hover:bg-amber-500/30 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          data-testid="hint-button"
          title={`${hints.length}/${totalHints} hints used`}
        >
          {hintLoading ? '...' : `Hint (${hints.length}/${totalHints})`}
        </button>
      </div>

      {/* Flag submission */}
      <div className="bg-surface rounded-lg p-3 space-y-2">
        <span className="text-xs text-fg-muted">Captured a flag?</span>
        <div className="flex gap-2">
          <input
            type="text"
            value={flagInput}
            onChange={e => setFlagInput(e.target.value)}
            onKeyDown={handleFlagKeyDown}
            placeholder="FLAG{...}"
            className="flex-1 bg-elevated border border-border-default rounded-lg px-3 py-2 text-sm text-fg placeholder:text-fg-muted focus:outline-none focus:border-soul font-mono"
            data-testid="flag-input"
          />
          <button
            onClick={handleSubmitFlag}
            disabled={!flagInput.trim()}
            className="px-4 py-2 text-sm rounded-lg bg-emerald-600 text-fg font-medium hover:bg-emerald-500 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            data-testid="submit-flag"
          >
            Submit Flag
          </button>
        </div>
        {flagMessage && (
          <div className={`text-xs px-3 py-1.5 rounded ${flagCorrect ? 'bg-emerald-500/10 text-emerald-400' : 'bg-red-500/10 text-red-400'}`} data-testid="flag-result">
            {flagMessage}
          </div>
        )}
      </div>
    </div>
  );
}
