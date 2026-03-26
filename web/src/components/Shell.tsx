import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import type { ConnectionState } from '../lib/types';
import { useChat } from '../hooks/useChat';
import { useSwipeDrawer } from '../hooks/useSwipeDrawer';
import { MessageList } from './MessageList';
import { ChatInput } from './ChatInput';
import type { ChatInputHandle } from './ChatInput';
import { SessionList } from './SessionList';
import { ConnectionBanner } from './ConnectionBanner';
import { SearchBar } from './SearchBar';

function connectionDotClasses(status: ConnectionState): string {
  switch (status) {
    case 'connected':
      return 'bg-green-500 animate-pulse';
    case 'connecting':
      return 'bg-yellow-500';
    case 'disconnected':
    case 'error':
      return 'bg-red-500';
  }
}

function connectionLabel(status: ConnectionState): string {
  switch (status) {
    case 'connected':
      return 'Connected';
    case 'connecting':
      return 'Connecting';
    case 'disconnected':
      return 'Disconnected';
    case 'error':
      return 'Error';
  }
}

export function Shell() {
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
  } = useChat();

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
    <div
      data-testid="shell"
      className="h-screen bg-deep text-fg flex flex-col noise"
      {...handlers}
    >
      {/* Header */}
      <header className="glass flex items-center justify-between px-4 h-11 shrink-0" role="banner">
        <div className="flex items-center gap-3">
          {/* Hamburger — visible on mobile only */}
          <button
            data-testid="sidebar-toggle"
            type="button"
            onClick={toggle}
            className="md:hidden p-1.5 -ml-1.5 rounded-lg text-fg-muted hover:text-fg hover:bg-elevated transition-colors"
            aria-label="Toggle sessions"
          >
            <svg width="18" height="18" viewBox="0 0 20 20" fill="none">
              <path
                d="M3 5h14M3 10h14M3 15h14"
                stroke="currentColor"
                strokeWidth="1.5"
                strokeLinecap="round"
              />
            </svg>
          </button>
          <span className="text-soul text-xl drop-shadow-[0_0_8px_rgba(232,168,73,0.4)]" aria-hidden="true">&#9670;</span>
          <h1 className="text-base font-semibold text-fg tracking-tight">Soul</h1>
        </div>
        <div className="flex items-center gap-3">
          {authError && (
            <button
              data-testid="reauth-button"
              onClick={reauth}
              className="px-2 py-1 text-xs rounded bg-red-700 hover:bg-red-600 text-fg transition-colors"
            >
              Re-authenticate
            </button>
          )}
          <div
            data-testid="connection-status"
            className="flex items-center gap-2 text-xs text-fg-muted"
            title={connectionLabel(status)}
            role="status"
            aria-live="polite"
            aria-label={`Connection: ${connectionLabel(status)}`}
          >
            <span className="hidden sm:inline">{connectionLabel(status)}</span>
            <span
              className={`inline-block h-2 w-2 rounded-full ${connectionDotClasses(status)}`}
            />
          </div>
        </div>
      </header>

      <ConnectionBanner status={status} reconnectAttempt={reconnectAttempt} />

      <div className="flex flex-1 min-h-0 relative">
        {/* Backdrop — mobile only, when drawer is open */}
        {isOpen && (
          <div
            data-testid="sidebar-backdrop"
            className="fixed inset-0 bg-black/60 z-30 md:hidden"
            onClick={close}
          />
        )}

        {/* Sidebar — overlay on mobile, static on desktop */}
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

        {/* Chat area */}
        <main className="flex-1 flex flex-col min-w-0">
          {searchOpen && (
            <SearchBar query={searchQuery} onChange={setSearchQuery} onClose={closeSearch} matchCount={matchCount} />
          )}
          <MessageList messages={messages} isStreaming={isStreaming} onSend={sendMessage} onEdit={editAndResend} onRetry={retryMessage} searchQuery={searchQuery} />
          <ChatInput ref={inputRef} onSend={sendMessage} onStop={stopGeneration} disabled={isDisabled} isStreaming={isStreaming} />
        </main>
      </div>
    </div>
  );
}
