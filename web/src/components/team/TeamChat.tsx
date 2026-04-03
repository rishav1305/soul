import { useState, useRef, useEffect, useCallback } from 'react';
import { useTeamChat } from '../../hooks/useTeamChat.ts';
import { ChatMessage } from './ChatMessage.tsx';

// Agent names for @mention autocomplete and DM routing
const AGENT_NAMES = ['fury', 'shuri', 'happy', 'xavier', 'pepper', 'loki', 'hawkeye', 'stark', 'banner'];
const AGENT_NAME_SET = new Set(AGENT_NAMES);

/**
 * extractMentionTarget — if a message begins with `@agentname` (whole word),
 * returns the agent name to use as the `to` field. Otherwise returns '*'.
 *
 * Exported for unit testing.
 *
 * Examples:
 *   "@shuri fix the bug"   → 'shuri'
 *   "@shuri"               → 'shuri'
 *   "hey @shuri look at this" → '*'  (broadcast, mention mid-sentence)
 *   "@unknownbot do this"  → '*'    (not a known agent)
 */
export function extractMentionTarget(content: string): string {
  const match = content.match(/^@(\w+)(?:\s|$)/);
  if (match?.[1] && AGENT_NAME_SET.has(match[1])) {
    return match[1];
  }
  return '*';
}

/**
 * TeamChat — chat interface with @mention autocomplete, conference mode, and slash commands.
 * Route: /team/chat
 */
export default function TeamChat() {
  const { messages, conferenceActive, sendMessage, startConference, endConference } = useTeamChat();
  const [input, setInput] = useState('');
  const [mentionQuery, setMentionQuery] = useState<string | null>(null);
  const [mentionIndex, setMentionIndex] = useState(0);
  const [conferenceTopic, setConferenceTopic] = useState('');
  const [showConferencePrompt, setShowConferencePrompt] = useState(false);
  const bottomRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);

  // Auto-scroll on new messages
  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // @mention detection
  const mentionSuggestions = mentionQuery !== null
    ? AGENT_NAMES.filter((n) => n.startsWith(mentionQuery))
    : [];

  const handleInputChange = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const val = e.target.value;
    setInput(val);

    // Detect @mention
    const match = val.match(/@(\w*)$/);
    if (match) {
      setMentionQuery(match[1] ?? '');
      setMentionIndex(0);
    } else {
      setMentionQuery(null);
    }
  }, []);

  const applyMention = useCallback((name: string) => {
    const replaced = input.replace(/@\w*$/, `@${name} `);
    setInput(replaced);
    setMentionQuery(null);
    inputRef.current?.focus();
  }, [input]);

  const handleSend = useCallback(() => {
    const trimmed = input.trim();
    if (!trimmed) return;

    // Slash command: /task Create: description
    if (trimmed.startsWith('/task ')) {
      const desc = trimmed.slice(6).replace(/^Create:\s*/i, '');
      sendMessage(`[TASK CREATED] ${desc}`, 'CEO');
      setInput('');
      return;
    }

    // Slash command: /conference topic
    if (trimmed.startsWith('/conference ')) {
      const topic = trimmed.slice(12);
      startConference(topic);
      setInput('');
      return;
    }

    sendMessage(trimmed, 'CEO', extractMentionTarget(trimmed));
    setInput('');
    setMentionQuery(null);
  }, [input, sendMessage, startConference]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent<HTMLInputElement>) => {
    if (mentionSuggestions.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setMentionIndex((i) => (i + 1) % mentionSuggestions.length);
        return;
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault();
        setMentionIndex((i) => (i - 1 + mentionSuggestions.length) % mentionSuggestions.length);
        return;
      }
      if (e.key === 'Tab' || e.key === 'Enter') {
        e.preventDefault();
        const chosen = mentionSuggestions[mentionIndex];
        if (chosen) applyMention(chosen);
        return;
      }
      if (e.key === 'Escape') {
        setMentionQuery(null);
        return;
      }
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  }, [mentionSuggestions, mentionIndex, applyMention, handleSend]);

  return (
    <div data-testid="team-chat" className="flex flex-col h-full bg-deep overflow-hidden">
      {/* ── Header ── */}
      <header className="flex items-center justify-between px-6 py-3 border-b border-border-subtle shrink-0">
        <div className="flex items-center gap-3">
          <h1 className="text-sm font-semibold text-fg">Team Chat</h1>
          {conferenceActive && (
            <span className="text-[10px] px-2 py-0.5 rounded-full bg-soul/20 text-soul border border-soul/30 font-medium">
              Conference Active
            </span>
          )}
        </div>

        <div className="flex items-center gap-2">
          {!conferenceActive ? (
            <button
              type="button"
              data-testid="conference-start-btn"
              onClick={() => setShowConferencePrompt(true)}
              className="px-3 py-1.5 text-xs border border-border-default text-fg-secondary hover:text-fg hover:border-border-active rounded transition-colors cursor-pointer"
            >
              Start Conference
            </button>
          ) : (
            <button
              type="button"
              data-testid="conference-end-btn"
              onClick={endConference}
              className="px-3 py-1.5 text-xs bg-red-900/40 text-red-400 hover:bg-red-900/60 rounded transition-colors cursor-pointer"
            >
              End Conference
            </button>
          )}
        </div>
      </header>

      {/* ── Conference prompt ── */}
      {showConferencePrompt && (
        <div className="flex items-center gap-2 px-6 py-2 border-b border-border-subtle bg-surface/60 shrink-0">
          <span className="text-xs text-fg-secondary shrink-0">Topic:</span>
          <input
            data-testid="conference-topic-input"
            type="text"
            value={conferenceTopic}
            onChange={(e) => setConferenceTopic(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && conferenceTopic.trim()) {
                startConference(conferenceTopic.trim());
                setConferenceTopic('');
                setShowConferencePrompt(false);
              }
              if (e.key === 'Escape') setShowConferencePrompt(false);
            }}
            placeholder="e.g. Sprint blockers review"
            className="flex-1 bg-deep border border-border-default rounded px-3 py-1.5 text-sm text-fg placeholder:text-fg-muted focus:outline-none focus:border-soul/50"
            autoFocus
          />
          <button
            type="button"
            data-testid="conference-start-confirm-btn"
            onClick={() => {
              if (conferenceTopic.trim()) {
                startConference(conferenceTopic.trim());
                setConferenceTopic('');
                setShowConferencePrompt(false);
              }
            }}
            className="px-3 py-1.5 text-xs bg-soul text-deep rounded hover:bg-soul/85 transition-colors cursor-pointer font-medium"
          >
            Start
          </button>
        </div>
      )}

      {/* ── Message list ── */}
      <div
        data-testid="chat-message-list"
        className="flex-1 overflow-y-auto py-3"
        aria-label="Chat messages"
        aria-live="polite"
      >
        {messages.map((msg) => (
          <ChatMessage key={msg.id} message={msg} />
        ))}
        <div ref={bottomRef} />
      </div>

      {/* ── Input area ── */}
      <div className="px-4 pb-4 pt-2 border-t border-border-subtle shrink-0 relative">
        {/* @mention autocomplete dropdown */}
        {mentionSuggestions.length > 0 && (
          <div
            data-testid="mention-autocomplete"
            className="absolute bottom-full left-4 right-4 mb-2 bg-elevated border border-border-default rounded-lg shadow-lg overflow-hidden"
            role="listbox"
            aria-label="Agent mentions"
          >
            {mentionSuggestions.map((name, i) => (
              <button
                key={name}
                type="button"
                data-testid={`mention-option-${name}`}
                role="option"
                aria-selected={i === mentionIndex}
                onClick={() => applyMention(name)}
                className={`w-full text-left px-3 py-2 text-sm transition-colors cursor-pointer ${
                  i === mentionIndex ? 'bg-soul/20 text-fg' : 'text-fg-secondary hover:bg-overlay/60'
                }`}
              >
                @{name}
              </button>
            ))}
          </div>
        )}

        <div className="flex items-center gap-2">
          <input
            ref={inputRef}
            data-testid="team-chat-input"
            type="text"
            value={input}
            onChange={handleInputChange}
            onKeyDown={handleKeyDown}
            placeholder="Message the team... (@mention, /task, /conference)"
            className="flex-1 bg-surface border border-border-default rounded-lg px-4 py-2.5 text-sm text-fg placeholder:text-fg-muted focus:outline-none focus:border-soul/50 transition-colors"
            aria-label="Chat message input"
            autoComplete="off"
          />
          <button
            type="button"
            data-testid="team-chat-send-btn"
            onClick={handleSend}
            disabled={!input.trim()}
            className="px-4 py-2.5 bg-soul text-deep text-sm font-medium rounded-lg hover:bg-soul/85 disabled:opacity-40 disabled:cursor-not-allowed transition-colors cursor-pointer"
            aria-label="Send message"
          >
            Send
          </button>
        </div>

        <p className="text-[10px] text-fg-muted mt-1.5 px-1">
          Use @name to mention an agent. /task Create: title to create a task. /conference topic to start a round.
        </p>
      </div>
    </div>
  );
}
