// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { renderHook, cleanup } from '@testing-library/react';
import type { ReactNode } from 'react';

// Mock useChat
vi.mock('../hooks/useChat', () => ({
  useChat: () => ({
    messages: [],
    isStreaming: false,
    status: 'connected',
    authError: false,
    reconnectAttempt: 0,
    sendMessage: vi.fn(),
    stopGeneration: vi.fn(),
    editAndResend: vi.fn(),
    retryMessage: vi.fn(),
    reauth: vi.fn(),
    reconnect: vi.fn(),
    sessions: [],
    currentSessionID: null,
    createSession: vi.fn(),
    switchSession: vi.fn(),
    deleteSession: vi.fn(),
    renameSession: vi.fn(),
    activeProduct: null,
    setProduct: vi.fn(),
  }),
}));

import { ChatProvider, useChatContext } from './ChatContext';

afterEach(() => cleanup());

function wrapper({ children }: { children: ReactNode }) {
  return <ChatProvider>{children}</ChatProvider>;
}

describe('ChatContext', () => {
  it('throws when used outside ChatProvider', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {});
    expect(() => {
      renderHook(() => useChatContext());
    }).toThrow('useChatContext must be used within ChatProvider');
    spy.mockRestore();
  });

  it('provides chat state inside ChatProvider', () => {
    const { result } = renderHook(() => useChatContext(), { wrapper });
    expect(result.current.messages).toEqual([]);
    expect(result.current.isStreaming).toBe(false);
    expect(result.current.status).toBe('connected');
    expect(result.current.currentSessionID).toBeNull();
  });
});
