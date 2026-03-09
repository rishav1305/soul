import type { ConnectionState } from '../lib/types';
import { useChat } from '../hooks/useChat';
import { MessageList } from './MessageList';
import { ChatInput } from './ChatInput';

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
  const { messages, isStreaming, status, sendMessage } = useChat();

  const isDisabled = isStreaming || status !== 'connected';

  return (
    <div
      data-testid="shell"
      className="h-screen bg-zinc-950 text-zinc-100 flex flex-col"
    >
      <header className="flex items-center justify-between px-4 py-3 border-b border-zinc-800">
        <h1 className="text-lg font-semibold tracking-tight">Soul v2</h1>
        <div
          data-testid="connection-status"
          className="flex items-center gap-2 text-xs text-zinc-400"
          title={connectionLabel(status)}
        >
          <span>{connectionLabel(status)}</span>
          <span
            className={`inline-block h-2 w-2 rounded-full ${connectionDotClasses(status)}`}
          />
        </div>
      </header>
      <MessageList messages={messages} isStreaming={isStreaming} />
      <ChatInput onSend={sendMessage} disabled={isDisabled} />
    </div>
  );
}
