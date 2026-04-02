// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { renderHook, cleanup, act } from '@testing-library/react';
import React from 'react';

// --- Mock WSClient ---

const mockConnect = vi.fn();
const mockDisconnect = vi.fn();
const mockSend = vi.fn();
const mockOnMessage = vi.fn();

vi.mock('../lib/ws-client.ts', () => {
  class MockWSClient {
    constructor(_urlFactory: unknown, _onConnected: unknown) {}
    connect = mockConnect;
    disconnect = mockDisconnect;
    send = mockSend;
    onMessage = mockOnMessage;
  }
  return { WSClient: MockWSClient };
});

vi.mock('../lib/ws.ts', () => ({
  fetchWSTicket: vi.fn().mockResolvedValue({ ticket: 'test-ticket', status: 200 }),
  getWebSocketURL: vi.fn().mockReturnValue('ws://localhost/ws?ticket=test-ticket'),
}));

import { useWebSocketProvider, useWebSocketCtx, WebSocketContext } from './useWebSocketContext';

describe('useWebSocketProvider', () => {
  beforeEach(() => {
    mockConnect.mockClear();
    mockDisconnect.mockClear();
    mockSend.mockClear();
    mockOnMessage.mockClear();
  });
  afterEach(() => cleanup());

  it('creates WSClient and connects on mount', () => {
    const { result } = renderHook(() => useWebSocketProvider());

    expect(mockConnect).toHaveBeenCalledTimes(1);
    expect(result.current).toHaveProperty('send');
    expect(result.current).toHaveProperty('onMessage');
    expect(result.current).toHaveProperty('connected');
  });

  it('disconnects on unmount', () => {
    const { unmount } = renderHook(() => useWebSocketProvider());
    unmount();
    expect(mockDisconnect).toHaveBeenCalledTimes(1);
  });

  it('returns connected as false initially', () => {
    const { result } = renderHook(() => useWebSocketProvider());
    expect(result.current.connected).toBe(false);
  });

  it('send delegates to WSClient', () => {
    const { result } = renderHook(() => useWebSocketProvider());

    act(() => {
      result.current.send({ type: 'test', data: 'hello' } as any);
    });

    expect(mockSend).toHaveBeenCalledWith({ type: 'test', data: 'hello' });
  });

  it('onMessage delegates to WSClient', () => {
    const unsub = vi.fn();
    mockOnMessage.mockReturnValue(unsub);

    const { result } = renderHook(() => useWebSocketProvider());

    const handler = vi.fn();
    let unsubscribe: (() => void) | undefined;
    act(() => {
      unsubscribe = result.current.onMessage(handler);
    });

    expect(mockOnMessage).toHaveBeenCalledWith(handler);
    expect(typeof unsubscribe).toBe('function');
  });

  it('onMessage returns noop when client is null', () => {
    // The client is always created eagerly, so this exercises the ?? fallback
    // only if WSClient constructor returns null — hard to trigger in practice.
    // At minimum, verify onMessage never throws.
    const { result } = renderHook(() => useWebSocketProvider());
    const handler = vi.fn();
    let unsub: (() => void) | undefined;
    act(() => { unsub = result.current.onMessage(handler); });
    expect(typeof unsub).toBe('function');
  });
});

describe('useWebSocketCtx', () => {
  afterEach(() => cleanup());

  it('throws when used outside provider', () => {
    expect(() => {
      renderHook(() => useWebSocketCtx());
    }).toThrow('useWebSocketCtx must be used within a WebSocketContext.Provider');
  });

  it('returns context value when inside provider', () => {
    const contextValue = {
      send: vi.fn(),
      onMessage: vi.fn(),
      connected: true,
    };

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(WebSocketContext.Provider, { value: contextValue }, children);

    const { result } = renderHook(() => useWebSocketCtx(), { wrapper });

    expect(result.current.send).toBe(contextValue.send);
    expect(result.current.onMessage).toBe(contextValue.onMessage);
    expect(result.current.connected).toBe(true);
  });

  it('returns connected=false via context', () => {
    const contextValue = {
      send: vi.fn(),
      onMessage: vi.fn(),
      connected: false,
    };

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(WebSocketContext.Provider, { value: contextValue }, children);

    const { result } = renderHook(() => useWebSocketCtx(), { wrapper });
    expect(result.current.connected).toBe(false);
  });

  it('send from context calls the provided send function', () => {
    const sendFn = vi.fn();
    const contextValue = {
      send: sendFn,
      onMessage: vi.fn(),
      connected: true,
    };

    const wrapper = ({ children }: { children: React.ReactNode }) =>
      React.createElement(WebSocketContext.Provider, { value: contextValue }, children);

    const { result } = renderHook(() => useWebSocketCtx(), { wrapper });
    act(() => {
      result.current.send({ type: 'chat.send', content: 'hello' } as any);
    });
    expect(sendFn).toHaveBeenCalledWith({ type: 'chat.send', content: 'hello' });
  });
});
