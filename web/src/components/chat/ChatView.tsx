import { useEffect, useRef, useState, useCallback } from 'react';
import { useChat } from '../../hooks/useChat.ts';
import Message from './Message.tsx';
import InputBar from './InputBar.tsx';
import type { SendOptions } from '../../lib/types.ts';

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
  const { messages, sendMessage, isStreaming, sessionId, setSessionId } = useChat();
  const scrollRef = useRef<HTMLDivElement>(null);
  // Track whether we've auto-injected for this session (empty → first message)
  const injectedRef = useRef(false);
  // Pending context to prepend to next send
  const pendingContextRef = useRef<string>('');
  // Track the previous product to detect navigation changes
  const prevProductRef = useRef<string | null | undefined>(activeProduct);
  // Whether to show the "inject context?" chip
  const [contextChipProduct, setContextChipProduct] = useState<string | null>(null);

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

  // Auto-scroll on new messages
  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

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

  const handleSend = useCallback((content: string, options?: SendOptions) => {
    const ctx = pendingContextRef.current;
    pendingContextRef.current = '';
    injectedRef.current = true;
    if (ctx) {
      sendMessage(content, { ...options, context: ctx });
    } else {
      sendMessage(content, options);
    }
  }, [sendMessage]);

  return (
    <div className="flex flex-col h-full">
      <div ref={scrollRef} className="flex-1 overflow-y-auto px-5 py-8">
        <div className="max-w-3xl mx-auto space-y-6">
          {messages.length === 0 && (
            <div className="text-center py-16">
              <p className="text-fg-muted text-sm">Start a conversation</p>
            </div>
          )}
          {messages.map((msg) => (
            <Message key={msg.id} message={msg} />
          ))}
          {isStreaming && messages[messages.length - 1]?.role !== 'assistant' && (
            <div className="flex gap-3">
              <div className="flex gap-1 items-center py-2 px-1">
                <span className="w-1.5 h-1.5 rounded-full bg-soul animate-bounce [animation-delay:0ms]" />
                <span className="w-1.5 h-1.5 rounded-full bg-soul animate-bounce [animation-delay:150ms]" />
                <span className="w-1.5 h-1.5 rounded-full bg-soul animate-bounce [animation-delay:300ms]" />
              </div>
            </div>
          )}
        </div>
      </div>
      <InputBar
        onSend={handleSend}
        disabled={isStreaming}
        contextChip={contextChipProduct}
        onInjectContext={handleInjectContext}
        onDismissChip={handleDismissChip}
      />
    </div>
  );
}
