import { useEffect, useRef } from 'react';
import { useChat } from '../../hooks/useChat.ts';
import Message from './Message.tsx';
import InputBar from './InputBar.tsx';

interface ContextChip {
  label: string;
  onInject: () => void;
  onDismiss: () => void;
}

interface ChatViewProps {
  contextChips?: ContextChip[];
  autoInjectContext?: boolean;
  contextString?: string;
}

export default function ChatView({
  contextChips = [],
  autoInjectContext = false,
  contextString = '',
}: ChatViewProps) {
  const { messages, sendMessage, isStreaming } = useChat();
  const scrollRef = useRef<HTMLDivElement>(null);
  // Track whether we've auto-injected for this session (empty → first message)
  const injectedRef = useRef(false);
  // Pending context to prepend to next send
  const pendingContextRef = useRef<string>('');

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

  useEffect(() => {
    if (scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
    }
  }, [messages]);

  const handleSend = (content: string, options?: import('../../lib/types.ts').SendOptions) => {
    const ctx = pendingContextRef.current;
    pendingContextRef.current = '';
    injectedRef.current = true;
    sendMessage(content, ctx ? { ...options, context: ctx } : options);
  };

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
      <InputBar onSend={handleSend} disabled={isStreaming} contextChips={contextChips} />
    </div>
  );
}
