import { useState, useEffect, useRef } from 'react';
import ChatView from './ChatView.tsx';
import ChatNavbar from './ChatNavbar.tsx';
import SessionDrawer from './SessionDrawer.tsx';
import { useChat } from '../../hooks/useChat.ts';
import { useSessions } from '../../hooks/useSessions.ts';

interface ChatPanelProps {
  onCollapse: () => void;
  canCollapse: boolean;
  onUnreadChange: (count: number) => void;
}

export default function ChatPanel({ onCollapse, canCollapse, onUnreadChange }: ChatPanelProps) {
  const { messages } = useChat();
  const { sessions, activeSessionId, createSession, switchSession } = useSessions();
  const [drawerOpen, setDrawerOpen] = useState(false);
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
        onToggleDrawer={() => setDrawerOpen(!drawerOpen)}
        onCollapse={onCollapse}
        canCollapse={canCollapse}
      />
      {drawerOpen && (
        <SessionDrawer
          sessions={sessions}
          activeSessionId={activeSessionId}
          onSelect={switchSession}
          onNew={async () => {
            await createSession();
            setDrawerOpen(false);
          }}
          onClose={() => setDrawerOpen(false)}
        />
      )}
      <div className="flex-1 overflow-hidden">
        <ChatView />
      </div>
    </div>
  );
}
