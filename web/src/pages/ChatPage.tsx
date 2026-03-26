import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { useSearchParams } from 'react-router';
import { useChatContext } from '../contexts/ChatContext';
import { usePerformance } from '../hooks/usePerformance';
import { reportUsage } from '../lib/telemetry';
import { MessageList } from '../components/MessageList';
import { ChatInput } from '../components/ChatInput';
import type { ChatInputHandle } from '../components/ChatInput';
import { ConnectionBanner } from '../components/ConnectionBanner';
import { SearchBar } from '../components/SearchBar';
import { ChatTopBar } from '../components/ChatTopBar';
import { SessionsPanel } from '../components/SessionsPanel';

const SESSIONS_KEY = 'soul-v2-sessions-open';

export function ChatPage() {
  usePerformance('ChatPage');
  const [searchParams, setSearchParams] = useSearchParams();
  useEffect(() => { reportUsage('page.view', { page: 'chat' }); }, []);
  const {
    messages,
    isStreaming,
    status,
    reconnectAttempt,
    reconnect,
    authError,
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
    activeProduct,
    setProduct,
  } = useChatContext();

  const inputRef = useRef<ChatInputHandle>(null);

  // Auto-select product from ?product= query param (sidebar chat-only links)
  useEffect(() => {
    const product = searchParams.get('product');
    if (product && product !== activeProduct) {
      setProduct(product);
      // Clear the query param to keep URL clean
      setSearchParams({}, { replace: true });
    }
  }, [searchParams, activeProduct, setProduct, setSearchParams]);

  const [sessionsOpen, setSessionsOpen] = useState(() => {
    const stored = localStorage.getItem(SESSIONS_KEY);
    if (stored !== null) return stored === 'true';
    return window.innerWidth >= 640;
  });

  const toggleSessions = useCallback(() => {
    setSessionsOpen(prev => {
      const next = !prev;
      localStorage.setItem(SESSIONS_KEY, String(next));
      return next;
    });
  }, []);

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

  return (
    <div data-testid="chat-page" className="h-dvh flex">
      {/* Chat content */}
      <main className="flex-1 flex flex-col min-w-0">
        <ChatTopBar
          onCreateSession={createSession}
          sessions={sessions}
          onSwitchSession={switchSession}
          sessionsOpen={sessionsOpen}
          onToggleSessions={toggleSessions}
        />
        <ConnectionBanner
          status={status}
          reconnectAttempt={reconnectAttempt}
          onReconnect={reconnect}
          authError={authError}
          onReauth={reauth}
        />
        {searchOpen && (
          <SearchBar query={searchQuery} onChange={setSearchQuery} onClose={closeSearch} matchCount={matchCount} />
        )}
        <MessageList messages={messages} isStreaming={isStreaming} onSend={sendMessage} onEdit={editAndResend} onRetry={retryMessage} searchQuery={searchQuery} />
        <ChatInput ref={inputRef} onSend={sendMessage} onStop={stopGeneration} disabled={isDisabled} isStreaming={isStreaming} activeProduct={activeProduct} onSetProduct={setProduct} />
      </main>

      {/* Right sessions panel */}
      <SessionsPanel
        open={sessionsOpen}
        sessions={sessions}
        activeSessionID={currentSessionID}
        onSwitch={switchSession}
        onDelete={deleteSession}
        onRename={renameSession}
        onClose={() => {
          setSessionsOpen(false);
          localStorage.setItem(SESSIONS_KEY, 'false');
        }}
      />
    </div>
  );
}
