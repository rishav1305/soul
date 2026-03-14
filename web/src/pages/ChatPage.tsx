import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { useChatContext } from '../contexts/ChatContext';
import { useSwipeDrawer } from '../hooks/useSwipeDrawer';
import { usePerformance } from '../hooks/usePerformance';
import { reportUsage } from '../lib/telemetry';
import { MessageList } from '../components/MessageList';
import { ChatInput } from '../components/ChatInput';
import type { ChatInputHandle } from '../components/ChatInput';
import { SessionList } from '../components/SessionList';
import { ConnectionBanner } from '../components/ConnectionBanner';
import { SearchBar } from '../components/SearchBar';

export function ChatPage() {
  usePerformance('ChatPage');
  useEffect(() => { reportUsage('page.view', { page: 'chat' }); }, []);
  const {
    messages,
    isStreaming,
    status,
    authError,
    reconnectAttempt,
    sendMessage,
    stopGeneration,
    editAndResend,
    retryMessage,
    reauth,
    sessions,
    currentSessionID,
    createSession,
    switchSession,
    deleteSession,
    renameSession,
  } = useChatContext();

  const { isOpen, close, toggle, handlers } = useSwipeDrawer();
  const inputRef = useRef<ChatInputHandle>(null);

  const [searchOpen, setSearchOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState('');

  const closeSearch = useCallback(() => {
    setSearchOpen(false);
    setSearchQuery('');
  }, []);

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === 'f') {
        e.preventDefault();
        setSearchOpen(true);
      }
      if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
        e.preventDefault();
        inputRef.current?.focus();
      }
      if ((e.metaKey || e.ctrlKey) && e.shiftKey && e.key === 'N') {
        e.preventDefault();
        createSession();
      }
      if (e.key === 'Escape' && searchOpen) {
        closeSearch();
      }
    };
    window.addEventListener('keydown', handler);
    return () => window.removeEventListener('keydown', handler);
  }, [searchOpen, closeSearch, createSession]);

  const matchCount = useMemo(() => {
    if (!searchQuery) return 0;
    const escaped = searchQuery.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
    const re = new RegExp(escaped, 'gi');
    return messages.reduce((n, msg) => n + (msg.content.match(re)?.length ?? 0), 0);
  }, [messages, searchQuery]);

  const isDisabled = status !== 'connected';

  const handleSwitch = (id: string) => {
    switchSession(id);
    close();
  };

  const handleCreate = () => {
    createSession();
    close();
  };

  return (
    <div data-testid="chat-page" className="h-full flex" {...handlers}>
      <ConnectionBanner status={status} reconnectAttempt={reconnectAttempt} />

      {/* Backdrop — mobile only */}
      {isOpen && (
        <div
          data-testid="sidebar-backdrop"
          className="fixed inset-0 bg-black/60 z-30 md:hidden"
          onClick={close}
        />
      )}

      {/* Sidebar */}
      <div
        data-testid="sidebar-drawer"
        className={`
          fixed inset-y-0 left-0 z-40 w-64
          transform transition-transform duration-200 ease-out
          md:relative md:translate-x-0 md:transition-none
          ${isOpen ? 'translate-x-0' : '-translate-x-full'}
        `}
      >
        <SessionList
          sessions={sessions}
          activeSessionID={currentSessionID}
          onCreate={handleCreate}
          onSwitch={handleSwitch}
          onDelete={deleteSession}
          onRename={renameSession}
        />
      </div>

      {/* Mobile sidebar toggle */}
      <button
        data-testid="sidebar-toggle"
        type="button"
        onClick={toggle}
        className="fixed bottom-4 left-4 z-50 md:hidden p-2 rounded-full bg-elevated text-fg-muted hover:text-fg shadow-lg"
        aria-label="Toggle sessions"
      >
        <svg width="18" height="18" viewBox="0 0 20 20" fill="none">
          <path d="M3 5h14M3 10h14M3 15h14" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" />
        </svg>
      </button>

      {/* Chat area */}
      <div className="flex-1 flex flex-col min-w-0">
        {searchOpen && (
          <SearchBar query={searchQuery} onChange={setSearchQuery} onClose={closeSearch} matchCount={matchCount} />
        )}
        <MessageList messages={messages} isStreaming={isStreaming} onSend={sendMessage} onEdit={editAndResend} onRetry={retryMessage} searchQuery={searchQuery} />
        <ChatInput ref={inputRef} onSend={sendMessage} onStop={stopGeneration} disabled={isDisabled} isStreaming={isStreaming} />
      </div>
    </div>
  );
}
