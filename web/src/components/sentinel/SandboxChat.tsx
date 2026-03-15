import { useState, useRef, useEffect } from 'react';
import type { AttackEntry, SandboxConfig } from '../../hooks/useSentinel';

interface SandboxChatProps {
  config: SandboxConfig;
  messages: AttackEntry[];
  onSend: (message: string) => Promise<void>;
}

export function SandboxChat({ config, messages, onSend }: SandboxChatProps) {
  const [input, setInput] = useState('');
  const [sending, setSending] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  const handleSend = async () => {
    if (!input.trim() || sending) return;
    const message = input.trim();
    setInput('');
    setSending(true);
    try {
      await onSend(message);
    } finally {
      setSending(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  return (
    <div className="flex flex-col h-full space-y-3" data-testid="sandbox-chat">
      {/* System prompt display */}
      {config.systemPrompt && (
        <div className="bg-surface rounded-lg p-3 border border-border-subtle">
          <div className="text-[10px] text-fg-muted uppercase tracking-wider mb-1">System Prompt</div>
          <p className="text-xs text-fg-secondary whitespace-pre-wrap">{config.systemPrompt}</p>
        </div>
      )}

      {/* Messages */}
      <div className="flex-1 overflow-y-auto space-y-2 min-h-0 bg-surface rounded-lg p-3" data-testid="sandbox-messages">
        {messages.length === 0 && (
          <div className="text-xs text-fg-muted text-center py-8">
            {config.name ? `Sandbox "${config.name}" ready. Send a message to begin.` : 'Configure the sandbox first, then start chatting.'}
          </div>
        )}
        {messages.map(entry => (
          <div
            key={entry.id}
            className={`rounded-lg px-3 py-2 text-sm ${
              entry.role === 'attacker'
                ? 'bg-elevated ml-8 text-fg'
                : 'bg-overlay mr-8 text-fg'
            }`}
          >
            <div className="text-[10px] text-fg-muted mb-1">
              {entry.role === 'attacker' ? 'You' : 'Target'}
            </div>
            <div className="whitespace-pre-wrap break-words">{entry.content}</div>
          </div>
        ))}
        <div ref={messagesEndRef} />
      </div>

      {/* Input */}
      <div className="flex gap-2">
        <input
          type="text"
          value={input}
          onChange={e => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="Send a message..."
          className="flex-1 bg-elevated border border-border-default rounded-lg px-3 py-2 text-sm text-fg placeholder:text-fg-muted focus:outline-none focus:border-soul"
          data-testid="sandbox-input"
          disabled={sending}
        />
        <button
          onClick={handleSend}
          disabled={sending || !input.trim()}
          className="px-4 py-2 text-sm rounded-lg bg-soul text-deep font-medium hover:bg-soul/85 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
          data-testid="sandbox-send"
        >
          Send
        </button>
      </div>
    </div>
  );
}
