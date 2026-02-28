import { useEffect, useRef } from 'react';
import ChatView from './ChatView.tsx';
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
      // New messages arrived while panel is open — reset unread
      onUnreadChange(0);
    }
    prevCountRef.current = messages.length;
  }, [messages.length, onUnreadChange]);

  return (
    <div className="flex flex-col h-full bg-zinc-950">
      {/* Navbar */}
      <div className="flex items-center gap-2 px-3 py-2 border-b border-zinc-800 shrink-0 h-11">
        {/* Hamburger placeholder — session drawer comes later */}
        <button
          type="button"
          className="text-zinc-500 hover:text-zinc-300 text-lg cursor-pointer"
          title="Sessions"
        >
          &#9776;
        </button>

        <span className="text-sm font-semibold text-zinc-100 flex items-center gap-1.5">
          <span className="text-zinc-400">&#9670;</span> Soul Chat
        </span>

        <div className="flex-1" />

        <button
          type="button"
          onClick={onCollapse}
          disabled={!canCollapse}
          className="text-zinc-500 hover:text-zinc-300 disabled:opacity-30 disabled:cursor-not-allowed text-sm font-mono cursor-pointer"
          title={canCollapse ? 'Collapse chat' : 'Cannot collapse — task panel is collapsed'}
        >
          [&minus;]
        </button>
      </div>

      {/* Body */}
      <div className="flex-1 overflow-hidden">
        <ChatView />
      </div>
    </div>
  );
}
