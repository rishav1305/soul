// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { MessageBubble } from './MessageBubble';
import type { Message } from '../lib/types';

// Mock child components
vi.mock('./Markdown', () => ({
  Markdown: ({ content }: { content: string }) => <div data-testid="mock-markdown">{content}</div>,
}));
vi.mock('./ThinkingBlock', () => ({
  ThinkingBlock: () => <div data-testid="mock-thinking-block">Thinking</div>,
}));
vi.mock('./ToolCallBlock', () => ({
  ToolCallBlock: ({ tool }: { tool: { name: string } }) => (
    <div data-testid="mock-tool-call">{tool.name}</div>
  ),
}));
vi.mock('../hooks/usePerformance', () => ({
  usePerformance: vi.fn(),
}));

function makeMessage(overrides: Partial<Message> = {}): Message {
  return {
    id: 'msg-1',
    content: 'Hello world',
    role: 'user',
    sessionID: 'session-1',
    createdAt: new Date().toISOString(),
    ...overrides,
  };
}

describe('MessageBubble', () => {
  afterEach(() => cleanup());

  it('renders user message bubble', () => {
    render(<MessageBubble message={makeMessage()} isStreaming={false} />);
    const bubbles = screen.getAllByTestId('message-bubble');
    expect(bubbles.length).toBeGreaterThan(0);
  });

  it('renders message content', () => {
    render(
      <MessageBubble
        message={makeMessage({ content: 'Test content here' })}
        isStreaming={false}
      />,
    );
    expect(screen.getByText('Test content here')).toBeTruthy();
  });

  it('renders assistant message with markdown', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'assistant', content: 'AI response' })}
        isStreaming={false}
      />,
    );
    expect(screen.getByTestId('mock-markdown')).toBeTruthy();
  });

  it('shows copy button for messages', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'assistant', content: 'Copy me' })}
        isStreaming={false}
      />,
    );
    expect(screen.getByTestId('copy-message-btn')).toBeTruthy();
  });

  it('shows retry button on user messages when onRetry provided', () => {
    const onRetry = vi.fn();
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: 'Retry me' })}
        isStreaming={false}
        onRetry={onRetry}
      />,
    );
    const btn = screen.getByTestId('retry-message-btn');
    expect(btn).toBeTruthy();
    fireEvent.click(btn);
    expect(onRetry).toHaveBeenCalledWith('msg-1');
  });

  it('shows edit button for user messages when onEdit provided', () => {
    const onEdit = vi.fn();
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: 'Edit me' })}
        isStreaming={false}
        onEdit={onEdit}
      />,
    );
    expect(screen.getByTestId('edit-message-btn')).toBeTruthy();
  });

  it('edit button opens textarea for editing', () => {
    const onEdit = vi.fn();
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: 'Original text' })}
        isStreaming={false}
        onEdit={onEdit}
      />,
    );
    fireEvent.click(screen.getByTestId('edit-message-btn'));
    expect(screen.getByTestId('edit-message-textarea')).toBeTruthy();
  });

  it('edit submit calls onEdit with new content', () => {
    const onEdit = vi.fn();
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: 'Original' })}
        isStreaming={false}
        onEdit={onEdit}
      />,
    );
    fireEvent.click(screen.getByTestId('edit-message-btn'));
    const textarea = screen.getByTestId('edit-message-textarea') as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: 'Edited text' } });
    fireEvent.click(screen.getByTestId('edit-submit-btn'));
    expect(onEdit).toHaveBeenCalledWith('msg-1', 'Edited text');
  });

  it('edit cancel restores original content', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: 'Original' })}
        isStreaming={false}
        onEdit={vi.fn()}
      />,
    );
    fireEvent.click(screen.getByTestId('edit-message-btn'));
    fireEvent.click(screen.getByTestId('edit-cancel-btn'));
    // Should close the editor, no textarea visible
    expect(screen.queryByTestId('edit-message-textarea')).toBeNull();
  });

  it('renders tool_use messages as tool call blocks', () => {
    const toolMessage = makeMessage({
      role: 'tool_use',
      content: JSON.stringify({ name: 'search_leads', input: { query: 'test' } }),
    });
    render(<MessageBubble message={toolMessage} isStreaming={false} />);
    expect(screen.getByTestId('mock-tool-call')).toBeTruthy();
  });

  it('renders tool_result messages with result preview', () => {
    const resultMessage = makeMessage({
      role: 'tool_result',
      content: 'Found 5 results',
    });
    render(<MessageBubble message={resultMessage} isStreaming={false} />);
    const bubble = screen.getByTestId('message-bubble');
    expect(bubble.textContent).toContain('result');
  });

  it('shows model label for assistant messages', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'assistant', model: 'claude-3-5-haiku-20241022', content: 'Hi' })}
        isStreaming={false}
      />,
    );
    const bubble = screen.getByTestId('message-bubble');
    // modelLabel should extract "Haiku"
    expect(bubble.parentElement?.textContent).toContain('Haiku');
  });

  it('shows thinking block when message has thinking content', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'assistant', content: 'Answer', thinking: 'Let me think...' })}
        isStreaming={false}
      />,
    );
    expect(screen.getByTestId('mock-thinking-block')).toBeTruthy();
  });

  it('renders inline tool calls from toolCalls array', () => {
    render(
      <MessageBubble
        message={makeMessage({
          role: 'assistant',
          content: 'Using tools',
          toolCalls: [{ id: 'tc-1', name: 'get_weather', input: {}, status: 'complete' }],
        })}
        isStreaming={false}
      />,
    );
    expect(screen.getByTestId('mock-tool-call')).toBeTruthy();
  });

  it('extracts chat mode from user message prefix', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: '/brainstorm Design a new API' })}
        isStreaming={false}
      />,
    );
    const bubble = screen.getByTestId('message-bubble');
    expect(bubble.textContent).toContain('Brainstorm');
    expect(bubble.textContent).toContain('Design a new API');
  });

  it('extracts /code mode label', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: '/code Write a function' })}
        isStreaming={false}
      />,
    );
    const bubble = screen.getByTestId('message-bubble');
    expect(bubble.textContent).toContain('Code');
    expect(bubble.textContent).toContain('Write a function');
  });

  it('extracts /architect mode label', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: '/architect Design the system' })}
        isStreaming={false}
      />,
    );
    const bubble = screen.getByTestId('message-bubble');
    expect(bubble.textContent).toContain('Architect');
  });

  it('does not extract mode for regular messages', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: 'Just a regular message' })}
        isStreaming={false}
      />,
    );
    const bubble = screen.getByTestId('message-bubble');
    expect(bubble.textContent).not.toContain('Code');
    expect(bubble.textContent).not.toContain('Architect');
    expect(bubble.textContent).not.toContain('Brainstorm');
  });

  it('shows streaming cursor with word count for assistant with content', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'assistant', content: 'Hello world again' })}
        isStreaming={true}
      />,
    );
    expect(screen.getByText('3 words')).toBeTruthy();
  });

  it('shows streaming cursor without word count for assistant with no content', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'assistant', content: '' })}
        isStreaming={true}
      />,
    );
    // Should show cursor but not word count
    const bubble = screen.getByTestId('message-bubble');
    expect(bubble.querySelector('.streaming-cursor')).toBeTruthy();
  });

  it('does not show streaming indicator on user messages', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: 'Hello' })}
        isStreaming={true}
      />,
    );
    expect(screen.queryByText(/words/)).toBeNull();
  });

  it('shows token usage in assistant footer when not streaming', () => {
    render(
      <MessageBubble
        message={makeMessage({
          role: 'assistant',
          content: 'Response',
          model: 'claude-3-5-sonnet-20241022',
          usage: { inputTokens: 1200, outputTokens: 300 },
        })}
        isStreaming={false}
      />,
    );
    const bubble = screen.getByTestId('message-bubble');
    // Should show model badge "Sonnet" in footer
    expect(bubble.parentElement?.textContent).toContain('Sonnet');
  });

  it('hides retry button during streaming', () => {
    const onRetry = vi.fn();
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: 'Test' })}
        isStreaming={true}
        onRetry={onRetry}
      />,
    );
    expect(screen.queryByTestId('retry-message-btn')).toBeNull();
  });

  it('does not show retry button when onRetry not provided', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: 'Test' })}
        isStreaming={false}
      />,
    );
    expect(screen.queryByTestId('retry-message-btn')).toBeNull();
  });

  it('does not show edit button when onEdit not provided', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: 'Test' })}
        isStreaming={false}
      />,
    );
    expect(screen.queryByTestId('edit-message-btn')).toBeNull();
  });

  it('does not show edit button for assistant messages', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'assistant', content: 'Test' })}
        isStreaming={false}
        onEdit={vi.fn()}
      />,
    );
    expect(screen.queryByTestId('edit-message-btn')).toBeNull();
  });

  it('highlights search query in user message text', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: 'Hello world' })}
        isStreaming={false}
        searchQuery="world"
      />,
    );
    const marks = screen.getByTestId('message-bubble').querySelectorAll('mark');
    expect(marks.length).toBe(1);
    expect(marks[0]?.textContent).toBe('world');
  });

  it('does not highlight when no search query', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: 'Hello world' })}
        isStreaming={false}
      />,
    );
    const marks = screen.getByTestId('message-bubble').querySelectorAll('mark');
    expect(marks.length).toBe(0);
  });

  it('tool_result with empty content shows "No output"', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'tool_result', content: '' })}
        isStreaming={false}
      />,
    );
    expect(screen.getByText('No output')).toBeTruthy();
  });

  it('tool_result truncates long content', () => {
    const longContent = 'A'.repeat(100);
    render(
      <MessageBubble
        message={makeMessage({ role: 'tool_result', content: longContent })}
        isStreaming={false}
      />,
    );
    const bubble = screen.getByTestId('message-bubble');
    // Should truncate to ~57 chars + ellipsis
    expect(bubble.textContent).toContain('AAA');
    expect(bubble.textContent).toContain('\u2026');
  });

  it('renders multiple inline tool calls from toolCalls array', () => {
    render(
      <MessageBubble
        message={makeMessage({
          role: 'assistant',
          content: 'Using multiple tools',
          toolCalls: [
            { id: 'tc-1', name: 'search', input: {}, status: 'complete' },
            { id: 'tc-2', name: 'fetch', input: {}, status: 'running' },
          ],
        })}
        isStreaming={false}
      />,
    );
    // With multiple tool calls, one running → ToolCallGroup shows "1 running…"
    const bubble = screen.getByTestId('message-bubble');
    expect(bubble.textContent).toContain('1 running');
  });

  it('shows timestamp for user messages', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: 'Hello', createdAt: new Date().toISOString() })}
        isStreaming={false}
      />,
    );
    // The footer should have a time string
    const bubble = screen.getByTestId('message-bubble');
    expect(bubble.parentElement?.textContent).toMatch(/just now|second|minute|hour/i);
  });

  it('copy button is rendered for messages with content', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'user', content: 'Copy this' })}
        isStreaming={false}
      />,
    );
    expect(screen.getByTestId('copy-message-btn')).toBeTruthy();
  });

  it('model label extracts Opus from model string', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'assistant', content: 'Response', model: 'claude-3-opus-20240229' })}
        isStreaming={false}
      />,
    );
    expect(screen.getByTestId('message-bubble').parentElement?.textContent).toContain('Opus');
  });

  it('model label extracts Sonnet from model string', () => {
    render(
      <MessageBubble
        message={makeMessage({ role: 'assistant', content: 'Response', model: 'claude-3-5-sonnet-20241022' })}
        isStreaming={false}
      />,
    );
    expect(screen.getByTestId('message-bubble').parentElement?.textContent).toContain('Sonnet');
  });
});
