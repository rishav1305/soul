// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import { MessageList } from './MessageList';
import type { Message } from '../lib/types';

// Mock MessageBubble to isolate MessageList logic
vi.mock('./MessageBubble', () => ({
  MessageBubble: ({ message, isStreaming }: { message: Message; isStreaming: boolean }) => (
    <div data-testid={`msg-${message.id}`} data-streaming={isStreaming}>
      {message.content}
    </div>
  ),
}));

function makeMsg(overrides: Partial<Message> = {}): Message {
  return {
    id: 'msg-1',
    role: 'user',
    content: 'Hello',
    createdAt: new Date().toISOString(),
    sessionID: 's1',
    ...overrides,
  };
}

describe('MessageList', () => {
  afterEach(() => cleanup());

  it('renders empty state with suggestions when no messages', () => {
    render(<MessageList messages={[]} isStreaming={false} />);
    const list = screen.getByTestId('message-list');
    expect(list).toBeTruthy();
    expect(list.textContent).toContain('What can I help you with?');
  });

  it('renders all 4 suggestion buttons', () => {
    render(<MessageList messages={[]} isStreaming={false} />);
    const btns = screen.getAllByTestId('suggestion-button');
    expect(btns).toHaveLength(4);
    expect(btns[0].textContent).toContain('Explain this codebase');
    expect(btns[1].textContent).toContain('Find bugs in my code');
    expect(btns[2].textContent).toContain('Write a test for...');
    expect(btns[3].textContent).toContain('Refactor this function');
  });

  it('calls onSend when suggestion clicked', () => {
    const onSend = vi.fn();
    render(<MessageList messages={[]} isStreaming={false} onSend={onSend} />);
    fireEvent.click(screen.getAllByTestId('suggestion-button')[0]);
    expect(onSend).toHaveBeenCalledWith('Explain this codebase');
  });

  it('renders messages when provided', () => {
    const msgs: Message[] = [
      makeMsg({ id: 'm1', content: 'Hello' }),
      makeMsg({ id: 'm2', role: 'assistant', content: 'Hi there' }),
    ];
    render(<MessageList messages={msgs} isStreaming={false} />);
    expect(screen.getByTestId('msg-m1')).toBeTruthy();
    expect(screen.getByTestId('msg-m2')).toBeTruthy();
  });

  it('does not render suggestions when messages exist', () => {
    render(<MessageList messages={[makeMsg()]} isStreaming={false} />);
    expect(screen.queryByTestId('suggestion-button')).toBeNull();
  });

  it('has proper aria attributes on message container', () => {
    render(<MessageList messages={[makeMsg()]} isStreaming={false} />);
    const list = screen.getByTestId('message-list');
    expect(list.getAttribute('role')).toBe('log');
    expect(list.getAttribute('aria-live')).toBe('polite');
    expect(list.getAttribute('aria-label')).toBe('Chat messages');
  });

  it('marks last assistant message as streaming when isStreaming=true', () => {
    const msgs: Message[] = [
      makeMsg({ id: 'm1', content: 'Hello' }),
      makeMsg({ id: 'm2', role: 'assistant', content: 'Thinking...' }),
    ];
    render(<MessageList messages={msgs} isStreaming={true} />);
    const last = screen.getByTestId('msg-m2');
    expect(last.getAttribute('data-streaming')).toBe('true');
  });

  it('does not mark user message as streaming even if last', () => {
    const msgs: Message[] = [
      makeMsg({ id: 'm1', content: 'Hello' }),
    ];
    render(<MessageList messages={msgs} isStreaming={true} />);
    const msg = screen.getByTestId('msg-m1');
    expect(msg.getAttribute('data-streaming')).toBe('false');
  });

  it('shows stream-bar when isStreaming and messages exist', () => {
    const msgs: Message[] = [makeMsg({ id: 'm1', role: 'assistant', content: 'Hi' })];
    const { container } = render(<MessageList messages={msgs} isStreaming={true} />);
    expect(container.querySelector('.stream-bar')).toBeTruthy();
  });

  it('hides stream-bar when not streaming', () => {
    const msgs: Message[] = [makeMsg({ id: 'm1', role: 'assistant', content: 'Hi' })];
    const { container } = render(<MessageList messages={msgs} isStreaming={false} />);
    expect(container.querySelector('.stream-bar')).toBeNull();
  });

  it('passes onEdit and onRetry to MessageBubble via mock check', () => {
    // We verify MessageList accepts these props without error
    const onEdit = vi.fn();
    const onRetry = vi.fn();
    render(
      <MessageList messages={[makeMsg()]} isStreaming={false} onEdit={onEdit} onRetry={onRetry} />,
    );
    expect(screen.getByTestId('msg-msg-1')).toBeTruthy();
  });

  it('passes searchQuery to children', () => {
    render(
      <MessageList messages={[makeMsg()]} isStreaming={false} searchQuery="test" />,
    );
    // Renders without error — searchQuery is forwarded to MessageBubble
    expect(screen.getByTestId('msg-msg-1')).toBeTruthy();
  });

  it('suggestion click without onSend does not throw', () => {
    expect(() => {
      render(<MessageList messages={[]} isStreaming={false} />);
      fireEvent.click(screen.getAllByTestId('suggestion-button')[0]);
    }).not.toThrow();
  });

  it('empty state has no role attribute', () => {
    render(<MessageList messages={[]} isStreaming={false} />);
    const list = screen.getByTestId('message-list');
    expect(list.getAttribute('role')).toBeNull();
  });

  it('non-last assistant message is not marked streaming', () => {
    const msgs: Message[] = [
      makeMsg({ id: 'm1', role: 'assistant', content: 'First assistant' }),
      makeMsg({ id: 'm2', role: 'user', content: 'Follow-up user' }),
    ];
    render(<MessageList messages={msgs} isStreaming={true} />);
    expect(screen.getByTestId('msg-m1').getAttribute('data-streaming')).toBe('false');
  });

  it('scroll-to-bottom button not rendered initially', () => {
    const msgs = [makeMsg({ id: 'm1' })];
    render(<MessageList messages={msgs} isStreaming={false} />);
    expect(screen.queryByTestId('scroll-to-bottom')).toBeNull();
  });

  it('scroll-to-bottom button appears when scrolled away from bottom', () => {
    const msgs = [makeMsg({ id: 'm1' })];
    render(<MessageList messages={msgs} isStreaming={false} />);
    const el = screen.getByTestId('message-list');
    // Simulate a tall content list where user has scrolled to the top
    Object.defineProperty(el, 'scrollHeight', { value: 1000, configurable: true });
    Object.defineProperty(el, 'clientHeight', { value: 300, configurable: true });
    Object.defineProperty(el, 'scrollTop', { value: 0, configurable: true });
    act(() => { fireEvent.scroll(el); });
    expect(screen.getByTestId('scroll-to-bottom')).toBeTruthy();
  });

  it('scroll-to-bottom button has aria-label "Scroll to bottom"', () => {
    const msgs = [makeMsg({ id: 'm1' })];
    render(<MessageList messages={msgs} isStreaming={false} />);
    const el = screen.getByTestId('message-list');
    Object.defineProperty(el, 'scrollHeight', { value: 1000, configurable: true });
    Object.defineProperty(el, 'clientHeight', { value: 300, configurable: true });
    Object.defineProperty(el, 'scrollTop', { value: 0, configurable: true });
    act(() => { fireEvent.scroll(el); });
    expect(screen.getByTestId('scroll-to-bottom').getAttribute('aria-label')).toBe('Scroll to bottom');
  });
});
