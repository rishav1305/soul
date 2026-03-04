import { useEffect, useRef } from 'react';
import ChatView from './ChatView.tsx';
import { useChat } from '../../hooks/useChat.ts';

interface ChatPanelProps {
  onUnreadChange?: (count: number) => void;
  activeSessionId: number | null;
  onSessionCreated?: (id: number) => void;
}

export default function ChatPanel({ onUnreadChange, activeSessionId, onSessionCreated }: ChatPanelProps) {
  const { messages } = useChat();
  const prevCountRef = useRef(messages.length);

  useEffect(() => {
    if (messages.length > prevCountRef.current) {
      onUnreadChange?.(0);
    }
    prevCountRef.current = messages.length;
  }, [messages.length, onUnreadChange]);

  return (
    <div className="flex flex-col h-full relative bg-surface">
      <div className="flex-1 overflow-hidden">
        <ChatView activeSessionId={activeSessionId} onSessionCreated={onSessionCreated} />
      </div>
    </div>
  );
}
