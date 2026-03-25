import { useEffect, useRef, useState, useCallback, useMemo } from 'react';
import { useChatSessions } from '../../hooks/useChatSessions.tsx';
import type { Message, SendOptions } from '../../lib/types.ts';
import { MessageList } from './MessageList.tsx';
import { ChatInput } from './ChatInput.tsx';

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
}

export default function ChatView({
  activeSessionId,
  onSessionCreated,
  activeProduct,
  buildContextString,
  autoInjectContext = true,
  showContextChip = true,
}: ChatViewProps) {
  const {
    messages,
    isStreaming,
    tokenUsage,
    sendMessage,
    stopStreaming,
    retryFromMessage,
    editMessage,
    activeSessionId: currentSessionId,
    setActiveSessionId,
  } = useChatSessions();

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

  // Build context string once (stable between renders)
  const contextString = useMemo(() => {
    if (!buildContextString || !autoInjectContext) return '';
    return buildContextString();
  }, [buildContextString, autoInjectContext]);

  // Adapt ChatMessage[] → Message[] for MessageList compatibility
  const adaptedMessages: Message[] = useMemo(() => {
    const sid = String(activeSessionId ?? '');
    return messages.map((m) => ({
      id: m.id,
      role: m.role as Message['role'],
      content: m.content,
      createdAt: m.timestamp instanceof Date ? m.timestamp.toISOString() : String(m.timestamp),
      sessionID: sid,
      model: m.model,
      thinking: m.thinking,
      toolCalls: m.toolCalls?.map((tc) => ({
        id: tc.id,
        name: tc.name,
        input: (typeof tc.input === 'object' && tc.input !== null ? tc.input : {}) as Record<string, unknown>,
        status: tc.status,
        output: tc.output,
        progress: tc.progress,
      })),
    }));
  }, [messages, activeSessionId]);

  // Filtered messages for search
  const filteredMessages = useMemo(() => {
    if (!searchQuery.trim()) return adaptedMessages;
    const q = searchQuery.toLowerCase();
    return adaptedMessages.filter((m) =>
      m.content.toLowerCase().includes(q) ||
      m.toolCalls?.some(tc => tc.name.includes(q) || tc.output?.toLowerCase().includes(q))
    );
  }, [adaptedMessages, searchQuery]);

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

  // Sync external activeSessionId → useChatSessions
  useEffect(() => {
    if (activeSessionId !== null && activeSessionId !== currentSessionId) {
      setActiveSessionId(activeSessionId);
    }
  }, [activeSessionId]); // eslint-disable-line

  // Notify parent when a new session is created
  useEffect(() => {
    if (currentSessionId !== null && currentSessionId !== activeSessionId && onSessionCreated) {
      onSessionCreated(currentSessionId);
    }
  }, [currentSessionId]); // eslint-disable-line

  // Keyboard shortcuts
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
    };
    document.addEventListener('keydown', handler);
    return () => document.removeEventListener('keydown', handler);
  }, [showSearch]);

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
  }, []);

  const handleSend = useCallback((content: string, options?: Partial<SendOptions>) => {
    const ctx = pendingContextRef.current;
    pendingContextRef.current = '';
    injectedRef.current = true;
    if (ctx) {
      sendMessage(content, { ...options, context: ctx });
    } else {
      sendMessage(content, options);
    }
  }, [sendMessage]);

  const handleRetry = useCallback((messageId: string) => {
    retryFromMessage(messageId);
  }, [retryFromMessage]);

  const handleEdit = useCallback((messageId: string, newContent: string) => {
    editMessage(messageId, newContent);
  }, [editMessage]);

  return (
    <div
      className="flex flex-col h-full relative"
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      data-testid="chat-view"
    >
      {isDragging && (
        <div className="absolute inset-0 z-50 flex items-center justify-center bg-deep/80 border-2 border-dashed border-soul rounded-lg">
          <span className="text-soul font-mono text-sm">Drop files to attach</span>
        </div>
      )}
      {showSearch && (
        <div className="px-5 py-2 border-b border-border-subtle flex items-center gap-2 shrink-0">
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
            data-testid="chat-search-input"
          />
          <span className="text-[10px] text-fg-muted font-mono">
            {searchQuery ? `${filteredMessages.length}/${messages.length}` : ''}
          </span>
          <button
            type="button"
            onClick={() => { setShowSearch(false); setSearchQuery(''); }}
            className="text-fg-muted hover:text-fg cursor-pointer"
            data-testid="chat-search-close"
          >
            <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
              <path d="M4 4l8 8M12 4l-8 8" />
            </svg>
          </button>
        </div>
      )}

      {/* Message list */}
      <MessageList
        messages={filteredMessages}
        isStreaming={isStreaming}
        onSend={handleSend}
        onEdit={handleEdit}
        onRetry={handleRetry}
        searchQuery={searchQuery}
      />

      {/* Context injection chip */}
      {contextChipProduct && (
        <div className="flex items-center gap-2 px-4 py-1.5 shrink-0">
          <button
            type="button"
            onClick={handleInjectContext}
            className="flex items-center gap-1 px-2 py-0.5 text-[10px] font-mono rounded-full bg-soul/10 text-soul hover:bg-soul/20 transition-colors cursor-pointer"
            data-testid="context-inject-chip"
          >
            <svg width="10" height="10" viewBox="0 0 16 16" fill="currentColor"><path d="M8 0L14 8L8 16L2 8Z" /></svg>
            Inject {contextChipProduct} context
          </button>
          <button
            type="button"
            onClick={handleDismissChip}
            className="text-fg-muted hover:text-fg text-[10px] cursor-pointer"
            data-testid="context-dismiss-chip"
          >
            Dismiss
          </button>
        </div>
      )}

      {/* Token usage display */}
      {!isStreaming && tokenUsage && (
        <div className="flex justify-center py-1 shrink-0">
          <span className="text-[10px] font-mono text-fg-muted px-2 py-0.5 rounded bg-elevated/50">
            {formatTokens(tokenUsage.inputTokens)} in &middot; {formatTokens(tokenUsage.outputTokens)} out
            {tokenUsage.contextPct > 0 && (
              <> &middot; <span className={tokenUsage.contextPct > 70 ? 'text-stage-blocked' : tokenUsage.contextPct > 50 ? 'text-stage-validation' : ''}>
                {tokenUsage.contextPct}% ctx
              </span></>
            )}
          </span>
        </div>
      )}

      {/* Input bar */}
      <div className="shrink-0">
        <ChatInput
          onSend={handleSend}
          onStop={() => stopStreaming()}
          disabled={false}
          isStreaming={isStreaming}
        />
      </div>
    </div>
  );
}
