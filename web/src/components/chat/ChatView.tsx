import { useEffect, useRef, useState, useCallback, useMemo } from 'react';
import { useChat } from '../../hooks/useChat.ts';
import Message from './Message.tsx';
import InputBar from './InputBar.tsx';
import type { SendOptions } from '../../lib/types.ts';

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`;
  return String(n);
}

interface ChatViewProps {
  activeSessionId: number | null;
  onSessionCreated?: (id: number) => void;
  // Context injection
  activeProduct?: string | null;
  buildContextString?: () => string;
  autoInjectContext?: boolean;
  showContextChip?: boolean;
  contextString?: string;
}

export default function ChatView({
  activeSessionId,
  onSessionCreated,
  activeProduct,
  buildContextString,
  autoInjectContext = true,
  showContextChip = true,
  contextString = '',
}: ChatViewProps) {
  const { messages, setMessages, sendMessage, isStreaming, sessionId, setSessionId, tokenUsage, retryFromMessage, editMessage, stopStreaming } = useChat();
  const scrollRef = useRef<HTMLDivElement>(null);
  // Track whether we've auto-injected for this session (empty → first message)
  const injectedRef = useRef(false);
  // Pending context to prepend to next send
  const pendingContextRef = useRef<string>('');
  // Track the previous product to detect navigation changes
  const prevProductRef = useRef<string | null | undefined>(activeProduct);
  // Whether to show the "inject context?" chip
  const [contextChipProduct, setContextChipProduct] = useState<string | null>(null);
  const [searchQuery, setSearchQuery] = useState('');
  const [showSearch, setShowSearch] = useState(false);
  const [isDragging, setIsDragging] = useState(false);
  const [droppedFiles, setDroppedFiles] = useState<File[]>([]);

  const filteredMessages = useMemo(() => {
    if (!searchQuery.trim()) return messages;
    const q = searchQuery.toLowerCase();
    return messages.filter((m) =>
      m.content.toLowerCase().includes(q) ||
      m.toolCalls?.some(tc => tc.name.includes(q) || tc.output?.toLowerCase().includes(q))
    );
  }, [messages, searchQuery]);

  // When messages is empty (new session), reset the injection gate
  useEffect(() => {
    if (messages.length === 0) {
      injectedRef.current = false;
      pendingContextRef.current = autoInjectContext && contextString
        ? contextString
        : '';
    }
  }, [messages.length, autoInjectContext, contextString]);

  // Keep pending context up to date when settings/product change on empty session
  useEffect(() => {
    if (messages.length === 0 && autoInjectContext && contextString) {
      pendingContextRef.current = contextString;
    }
  }, [contextString, autoInjectContext, messages.length]);

  // Sync external activeSessionId → useChat's sessionId
  useEffect(() => {
    if (activeSessionId !== null && activeSessionId !== sessionId) {
      setSessionId(activeSessionId);
    }
  }, [activeSessionId]); // eslint-disable-line

  // Notify parent when useChat creates a new session
  useEffect(() => {
    if (sessionId !== null && sessionId !== activeSessionId && onSessionCreated) {
      onSessionCreated(sessionId);
    }
  }, [sessionId]); // eslint-disable-line

  // Scroll-to-bottom tracking
  const [showScrollBtn, setShowScrollBtn] = useState(false);
  const isNearBottomRef = useRef(true);

  useEffect(() => {
    const el = scrollRef.current;
    if (!el) return;
    const handleScroll = () => {
      const nearBottom = el.scrollHeight - el.scrollTop - el.clientHeight < 80;
      isNearBottomRef.current = nearBottom;
      setShowScrollBtn(!nearBottom);
    };
    el.addEventListener('scroll', handleScroll, { passive: true });
    return () => el.removeEventListener('scroll', handleScroll);
  }, []);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 'f') {
        e.preventDefault();
        setShowSearch(true);
      }
      if (e.key === 'Escape' && showSearch) {
        setShowSearch(false);
        setSearchQuery('');
      }
      if ((e.ctrlKey || e.metaKey) && e.key === 'k') {
        e.preventDefault();
        const textarea = document.querySelector('textarea[placeholder*="Message"]') as HTMLTextAreaElement;
        if (textarea) {
          textarea.focus();
          textarea.value = '/';
          textarea.dispatchEvent(new Event('input', { bubbles: true }));
        }
      }
      if (e.key === 'n' && (e.ctrlKey || e.metaKey) && e.shiftKey) {
        e.preventDefault();
        setSessionId(null);
        setMessages([]);
      }
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [showSearch, setSessionId, setMessages]);

  // Auto-scroll on new messages only when near bottom
  useEffect(() => {
    if (scrollRef.current && isNearBottomRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  const scrollToBottom = useCallback(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTo({ top: scrollRef.current.scrollHeight, behavior: 'smooth' });
    }
  }, []);

  // Context injection logic
  useEffect(() => {
    if (!buildContextString) return;
    const prevProduct = prevProductRef.current;
    const productChanged = activeProduct !== prevProduct;
    prevProductRef.current = activeProduct;

    if (!productChanged) return;

    const isNewChat = messages.length === 0;

    if (isNewChat && autoInjectContext && activeProduct) {
      // Auto-inject: store context to be prepended to the first message
      pendingContextRef.current = buildContextString();
      setContextChipProduct(null);
    } else if (!isNewChat && showContextChip && activeProduct) {
      // Existing chat: show chip prompt
      setContextChipProduct(activeProduct);
    }
  }, [activeProduct]); // eslint-disable-line

  const handleInjectContext = useCallback(() => {
    if (!buildContextString) return;
    pendingContextRef.current = buildContextString();
    setContextChipProduct(null);
  }, [buildContextString]);

  const handleDismissChip = useCallback(() => {
    setContextChipProduct(null);
  }, []);

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  }, []);

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    if (e.currentTarget === e.target) setIsDragging(false);
  }, []);

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    const files = Array.from(e.dataTransfer.files);
    if (files.length > 0) setDroppedFiles(files);
  }, []);

  const handleSend = useCallback((content: string, options?: SendOptions) => {
    const ctx = pendingContextRef.current;
    pendingContextRef.current = '';
    injectedRef.current = true;
    // Force scroll to bottom when user sends a message
    isNearBottomRef.current = true;
    if (ctx) {
      sendMessage(content, { ...options, context: ctx });
    } else {
      sendMessage(content, options);
    }
  }, [sendMessage]);

  return (
    <div className="flex flex-col h-full relative"
      onDragOver={handleDragOver} onDragLeave={handleDragLeave} onDrop={handleDrop}>
      {isDragging && (
        <div className="absolute inset-0 z-50 flex items-center justify-center bg-deep/80 border-2 border-dashed border-soul rounded-lg">
          <span className="text-soul font-mono text-sm">Drop files to attach</span>
        </div>
      )}
      {showSearch && (
        <div className="px-5 py-2 border-b border-border-subtle flex items-center gap-2">
          <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="var(--color-fg-muted)" strokeWidth="1.5">
            <circle cx="6.5" cy="6.5" r="5" /><path d="M11 11l3.5 3.5" />
          </svg>
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Search messages..."
            className="flex-1 bg-transparent text-sm text-fg placeholder:text-fg-muted focus:outline-none"
            autoFocus
          />
          <span className="text-[10px] text-fg-muted font-mono">
            {searchQuery ? `${filteredMessages.length}/${messages.length}` : ''}
          </span>
          <button type="button" onClick={() => { setShowSearch(false); setSearchQuery(''); }}
            className="text-fg-muted hover:text-fg cursor-pointer">
            <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
              <path d="M4 4l8 8M12 4l-8 8" />
            </svg>
          </button>
        </div>
      )}
      <div ref={scrollRef} className="flex-1 overflow-y-auto px-5 pt-1 pb-2">
        <div className="max-w-3xl mx-auto space-y-6">
          {messages.length === 0 && (
            <div className="flex flex-col items-center py-12 gap-6">
              <span className="text-5xl text-soul leading-none animate-float-glow">&#9670;</span>
              <p className="text-fg-muted text-sm">What can I help you with?</p>
              <div className="flex flex-wrap gap-2 justify-center max-w-md">
                {[
                  'Explain this codebase',
                  'Find bugs in my code',
                  'Write a test for...',
                  'Refactor this function',
                ].map((prompt) => (
                  <button
                    key={prompt}
                    type="button"
                    onClick={() => handleSend(prompt)}
                    className="px-3 py-1.5 text-xs font-mono rounded-full border border-border-subtle text-fg-muted hover:text-fg hover:border-soul/40 hover:bg-soul/5 transition-colors cursor-pointer"
                  >
                    {prompt}
                  </button>
                ))}
              </div>
            </div>
          )}
          {filteredMessages.map((msg) => (
            <Message key={msg.id} message={msg} onRetry={retryFromMessage} onEdit={editMessage} isStreaming={isStreaming} searchQuery={searchQuery} />
          ))}
          {isStreaming && (() => {
            const lastMsg = messages[messages.length - 1];
            if (lastMsg?.role === 'assistant' && lastMsg.content) {
              const words = lastMsg.content.trim().split(/\s+/).length;
              return (
                <div className="flex items-center gap-2 py-2 px-1">
                  <span className="w-1.5 h-1.5 rounded-full bg-soul animate-pulse" />
                  <span className="text-[10px] font-mono text-fg-muted">
                    {words} word{words !== 1 ? 's' : ''}
                  </span>
                </div>
              );
            }
            if (lastMsg?.role !== 'assistant') {
              return (
                <div className="flex items-center gap-3 py-3 px-1">
                  <div className="w-6 h-6 rounded-full bg-soul/15 flex items-center justify-center diamond-pulse diamond-breathe">
                    <svg width="10" height="10" viewBox="0 0 16 16" fill="var(--color-soul)">
                      <path d="M8 0L14 8L8 16L2 8Z" />
                    </svg>
                  </div>
                  <span className="text-[11px] font-mono text-fg-muted">
                    Soul is thinking<span className="animate-pulse">...</span>
                  </span>
                </div>
              );
            }
            return null;
          })()}
          {!isStreaming && tokenUsage && (
            <div className="flex justify-center">
              <span className="text-[10px] font-mono text-fg-muted px-2 py-0.5 rounded bg-elevated/50">
                {formatTokens(tokenUsage.inputTokens)} in · {formatTokens(tokenUsage.outputTokens)} out
                {tokenUsage.contextPct > 0 && (
                  <> · <span className={tokenUsage.contextPct > 70 ? 'text-stage-blocked' : tokenUsage.contextPct > 50 ? 'text-stage-validation' : ''}>
                    {tokenUsage.contextPct}% ctx
                  </span></>
                )}
              </span>
            </div>
          )}
        </div>
      </div>
      {/* Scroll-to-bottom FAB */}
      {showScrollBtn && (
        <div className="flex justify-center -mt-4 mb-1">
          <button
            type="button"
            onClick={scrollToBottom}
            className="flex items-center gap-1 px-3 py-1 rounded-full bg-elevated border border-border-subtle text-fg-muted hover:text-fg hover:bg-overlay transition-colors text-xs font-mono shadow-lg cursor-pointer"
          >
            <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="M8 3v10M4 9l4 4 4-4" />
            </svg>
            Latest
          </button>
        </div>
      )}
      {/* Streaming shimmer bar */}
      {isStreaming && <div className="stream-bar shrink-0" />}
      <InputBar
        onSend={handleSend}
        disabled={false}
        isStreaming={isStreaming}
        onStop={stopStreaming}
        contextChip={contextChipProduct}
        onInjectContext={handleInjectContext}
        onDismissChip={handleDismissChip}
        droppedFiles={droppedFiles}
        onDroppedFilesConsumed={() => setDroppedFiles([])}
      />
    </div>
  );
}
