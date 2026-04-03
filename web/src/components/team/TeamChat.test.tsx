// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { extractMentionTarget } from './TeamChat';

// ── Mock hooks ────────────────────────────────────────────────────────────────

const sendMessageMock = vi.fn();

vi.mock('../../hooks/useTeamChat.ts', () => ({
  useTeamChat: () => ({
    messages: [],
    conferenceActive: false,
    sendMessage: sendMessageMock,
    startConference: vi.fn(),
    endConference: vi.fn(),
  }),
}));

vi.mock('./ChatMessage.tsx', () => ({
  ChatMessage: ({ message }: { message: { content: string } }) => (
    <div data-testid="mock-chat-message">{message.content}</div>
  ),
}));

// ── extractMentionTarget — pure unit tests ─────────────────────────────────

describe('extractMentionTarget', () => {
  it('routes @agentname at start to that agent', () => {
    expect(extractMentionTarget('@shuri fix the bug')).toBe('shuri');
  });

  it('routes @agentname-only message to that agent', () => {
    expect(extractMentionTarget('@happy')).toBe('happy');
  });

  it('routes broadcast for mid-sentence @mention', () => {
    expect(extractMentionTarget('hey @shuri look at this')).toBe('*');
  });

  it('routes broadcast for unknown agent name', () => {
    expect(extractMentionTarget('@unknownbot do this')).toBe('*');
  });

  it('routes broadcast for empty string', () => {
    expect(extractMentionTarget('')).toBe('*');
  });

  it('routes broadcast for plain message', () => {
    expect(extractMentionTarget('hello team')).toBe('*');
  });

  it('routes all known agents correctly', () => {
    const agents = ['fury', 'shuri', 'happy', 'xavier', 'pepper', 'loki', 'hawkeye', 'stark', 'banner'];
    for (const agent of agents) {
      expect(extractMentionTarget(`@${agent} do something`)).toBe(agent);
    }
  });

  it('does not route @SHURI (uppercase — agents are lowercase)', () => {
    // Agent names are lowercase — uppercase mention should broadcast
    expect(extractMentionTarget('@SHURI fix this')).toBe('*');
  });

  it('routes broadcast for /task command', () => {
    expect(extractMentionTarget('/task Create: something')).toBe('*');
  });

  it('routes broadcast for /conference command', () => {
    expect(extractMentionTarget('/conference strategy review')).toBe('*');
  });
});

// ── TeamChat component — integration tests ─────────────────────────────────

describe('TeamChat component', () => {
  afterEach(() => {
    sendMessageMock.mockClear();
    cleanup();
  });

  async function importTeamChat() {
    const mod = await import('./TeamChat');
    return mod.default;
  }

  it('renders the chat input', async () => {
    const TeamChat = await importTeamChat();
    render(<TeamChat />);
    expect(screen.getByTestId('team-chat-input')).toBeTruthy();
  });

  it('sends broadcast when message has no leading @mention', async () => {
    const TeamChat = await importTeamChat();
    render(<TeamChat />);
    const input = screen.getByTestId('team-chat-input') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'hello everyone' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(sendMessageMock).toHaveBeenCalledWith('hello everyone', 'CEO', '*');
  });

  it('sends DM when message starts with @agentname', async () => {
    const TeamChat = await importTeamChat();
    render(<TeamChat />);
    const input = screen.getByTestId('team-chat-input') as HTMLInputElement;
    fireEvent.change(input, { target: { value: '@shuri fix this' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(sendMessageMock).toHaveBeenCalledWith('@shuri fix this', 'CEO', 'shuri');
  });

  it('sends broadcast when @mention is mid-sentence', async () => {
    const TeamChat = await importTeamChat();
    render(<TeamChat />);
    const input = screen.getByTestId('team-chat-input') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'hey @shuri look at this' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(sendMessageMock).toHaveBeenCalledWith('hey @shuri look at this', 'CEO', '*');
  });

  it('clears the input after sending', async () => {
    const TeamChat = await importTeamChat();
    render(<TeamChat />);
    const input = screen.getByTestId('team-chat-input') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'hello' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(input.value).toBe('');
  });

  it('does not call sendMessage on empty input', async () => {
    const TeamChat = await importTeamChat();
    render(<TeamChat />);
    const input = screen.getByTestId('team-chat-input') as HTMLInputElement;
    fireEvent.change(input, { target: { value: '   ' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(sendMessageMock).not.toHaveBeenCalled();
  });
});
