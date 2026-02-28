import { useEffect, useRef } from 'react';
import ChatView from './ChatView.tsx';
import ChatNavbar from './ChatNavbar.tsx';
import { useChat } from '../../hooks/useChat.ts';

interface ChatPanelProps {
  onCollapse: () => void;
  canCollapse: boolean;
  onUnreadChange: (count: number) => void;
}

export default function ChatPanel({ onCollapse, canCollapse, onUnreadChange }: ChatPanelProps) {
  const { messages } = useChat();
  const prevCountRef = useRef(messages.length);

  // Track unread: when panel is mounted, new messages reset to 0.
  // But we notify parent of incoming messages for when the panel is collapsed.
  useEffect(() => {
    if (messages.length > prevCountRef.current) {
      onUnreadChange(0);
    }
    prevCountRef.current = messages.length;
  }, [messages.length, onUnreadChange]);

  return (
    <div className="flex flex-col h-full relative bg-surface">
      <ChatNavbar
        onCollapse={onCollapse}
        canCollapse={canCollapse}
      />
      <div className="flex-1 overflow-hidden">
        <ChatView />
      </div>
    </div>
  );
}
