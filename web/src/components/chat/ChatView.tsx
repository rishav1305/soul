import { useEffect, useRef, useState, useCallback } from 'react';
import { useChat } from '../../hooks/useChat.ts';
import Message from './Message.tsx';
import InputBar from './InputBar.tsx';

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
  const { messages, sendMessage, isStreaming, sessionId, setSessionId } = useChat();
  const scrollRef = useRef<HTMLDivElement>(null);

  // Track the previous product to detect navigation changes
  const prevProductRef = useRef<string | null | undefined>(activeProduct);
  // Pending context to prepend to the next outgoing message
  const pendingContextRef = useRef<string | null>(null);
  // Whether to show the "inject context?" chip
  const [contextChipProduct, setContextChipProduct] = useState<string | null>(null);

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

  const handleSend = useCallback((content: string, options?: Parameters<typeof sendMessage>[1]) => {
    // Prepend any pending context to the message options
    const ctx = pendingContextRef.current;
    pendingContextRef.current = null;
    if (ctx) {
      sendMessage(content, { ...options, context: ctx });
    } else {
      sendMessage(content, options);
    }
  }, [sendMessage]);

  return (
    <div className="flex flex-col h-full">
      <div ref={scrollRef} className="flex-1 overflow-y-auto px-5 py-8">
        <div className="max-w-3xl mx-auto space-y-4">
          {messages.length === 0 && (
            <div className="flex items-center justify-center h-full min-h-[200px]">
              <div className="text-center">
                <div className="relative inline-block">
                  <div className="absolute inset-0 -m-8 bg-soul/15 rounded-full blur-3xl animate-soul-pulse" />
                  <div className="relative text-8xl text-soul animate-float">&#9670;</div>
                </div>
                <p className="font-display text-xl text-fg-secondary mt-6">How can I help you?</p>
                {activeProduct && autoInjectContext && (
                  <p className="text-xs text-fg-muted mt-2">
                    Context from <span className="text-soul">{activeProduct}</span> will be included
                  </p>
                )}
              </div>
            </div>
          )}
          {messages.map((msg) => (
            <Message key={msg.id} message={msg} />
          ))}
          {isStreaming && (
            <div className="flex justify-start">
              <div className="text-fg-muted text-sm font-body italic px-4 py-2">
                Soul is thinking...
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
