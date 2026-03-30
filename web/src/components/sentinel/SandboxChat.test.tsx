// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import { SandboxChat } from './SandboxChat';

// scrollIntoView not implemented in jsdom
window.HTMLElement.prototype.scrollIntoView = vi.fn();

function makeConfig(overrides: Record<string, unknown> = {}) {
  return {
    name: 'Test Sandbox',
    systemPrompt: 'You are a helpful assistant.',
    guardrails: [],
    weaknessLevel: 'medium' as const,
    ...overrides,
  };
}

function makeMessage(overrides: Record<string, unknown> = {}) {
  return {
    id: 'm1',
    role: 'attacker' as const,
    content: 'Hello',
    ...overrides,
  };
}

describe('SandboxChat', () => {
  afterEach(() => cleanup());

  it('renders sandbox chat container', () => {
    render(<SandboxChat config={makeConfig()} messages={[]} onSend={vi.fn()} />);
    expect(screen.getByTestId('sandbox-chat')).toBeTruthy();
  });

  it('shows system prompt', () => {
    render(<SandboxChat config={makeConfig()} messages={[]} onSend={vi.fn()} />);
    expect(screen.getByText('You are a helpful assistant.')).toBeTruthy();
  });

  it('shows System Prompt label', () => {
    render(<SandboxChat config={makeConfig()} messages={[]} onSend={vi.fn()} />);
    expect(screen.getByText('System Prompt')).toBeTruthy();
  });

  it('hides system prompt when empty', () => {
    render(<SandboxChat config={makeConfig({ systemPrompt: '' })} messages={[]} onSend={vi.fn()} />);
    expect(screen.queryByText('System Prompt')).toBeNull();
  });

  it('renders messages area', () => {
    render(<SandboxChat config={makeConfig()} messages={[]} onSend={vi.fn()} />);
    expect(screen.getByTestId('sandbox-messages')).toBeTruthy();
  });

  it('shows empty state with sandbox name', () => {
    render(<SandboxChat config={makeConfig({ name: 'My Sandbox' })} messages={[]} onSend={vi.fn()} />);
    expect(screen.getByText('Sandbox "My Sandbox" ready. Send a message to begin.')).toBeTruthy();
  });

  it('shows configure message when no name', () => {
    render(<SandboxChat config={makeConfig({ name: '' })} messages={[]} onSend={vi.fn()} />);
    expect(screen.getByText('Configure the sandbox first, then start chatting.')).toBeTruthy();
  });

  it('renders message entries', () => {
    const messages = [
      makeMessage({ id: 'm1', role: 'attacker', content: 'Hi there' }),
      makeMessage({ id: 'm2', role: 'defender', content: 'Hello back' }),
    ];
    render(<SandboxChat config={makeConfig()} messages={messages} onSend={vi.fn()} />);
    expect(screen.getByText('Hi there')).toBeTruthy();
    expect(screen.getByText('Hello back')).toBeTruthy();
    expect(screen.getByText('You')).toBeTruthy();
    expect(screen.getByText('Target')).toBeTruthy();
  });

  it('renders input', () => {
    render(<SandboxChat config={makeConfig()} messages={[]} onSend={vi.fn()} />);
    expect(screen.getByTestId('sandbox-input')).toBeTruthy();
  });

  it('renders send button', () => {
    render(<SandboxChat config={makeConfig()} messages={[]} onSend={vi.fn()} />);
    expect(screen.getByTestId('sandbox-send')).toBeTruthy();
  });

  it('send disabled when input empty', () => {
    render(<SandboxChat config={makeConfig()} messages={[]} onSend={vi.fn()} />);
    expect((screen.getByTestId('sandbox-send') as HTMLButtonElement).disabled).toBe(true);
  });

  it('calls onSend when send clicked', async () => {
    const onSend = vi.fn().mockResolvedValue(undefined);
    render(<SandboxChat config={makeConfig()} messages={[]} onSend={onSend} />);
    fireEvent.change(screen.getByTestId('sandbox-input'), { target: { value: 'test message' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('sandbox-send'));
    });
    expect(onSend).toHaveBeenCalledWith('test message');
  });

  it('clears input after send', async () => {
    const onSend = vi.fn().mockResolvedValue(undefined);
    render(<SandboxChat config={makeConfig()} messages={[]} onSend={onSend} />);
    fireEvent.change(screen.getByTestId('sandbox-input'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('sandbox-send'));
    });
    expect((screen.getByTestId('sandbox-input') as HTMLInputElement).value).toBe('');
  });
});
