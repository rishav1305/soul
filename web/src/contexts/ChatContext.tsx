import { createContext, useContext } from 'react';
import type { ReactNode } from 'react';
import { useChat } from '../hooks/useChat';
import type { Message, Session, ConnectionState, ChatProduct, ThinkingConfig } from '../lib/types';

interface ChatContextValue {
  messages: Message[];
  isStreaming: boolean;
  status: ConnectionState;
  authError: boolean;
  reconnectAttempt: number;
  sendMessage: (content: string, options?: { model?: string; thinking?: ThinkingConfig; attachments?: { name: string; mediaType: string; data: string }[] }) => void;
  stopGeneration: () => void;
  editAndResend: (messageId: string, newContent: string) => void;
  retryMessage: (messageId: string) => void;
  reauth: () => Promise<void>;
  reconnect: () => void;
  sessions: Session[];
  currentSessionID: string | null;
  createSession: () => void;
  switchSession: (id: string) => void;
  deleteSession: (id: string) => void;
  renameSession: (id: string, title: string) => void;
  activeProduct: ChatProduct;
  setProduct: (product: ChatProduct) => void;
}

const ChatContext = createContext<ChatContextValue | null>(null);

export function ChatProvider({ children }: { children: ReactNode }) {
  const chat = useChat();
  return <ChatContext.Provider value={chat}>{children}</ChatContext.Provider>;
}

export function useChatContext(): ChatContextValue {
  const ctx = useContext(ChatContext);
  if (!ctx) throw new Error('useChatContext must be used within ChatProvider');
  return ctx;
}
