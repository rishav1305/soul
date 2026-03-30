// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach, beforeEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ChatInput } from './ChatInput';

// Mock useModels hook
vi.mock('../hooks/useModels', () => ({
  useModels: () => ({
    models: [{ id: 'claude-3-5-haiku-20241022', name: 'Claude Haiku 4.5', max_tokens: 8192 }],
    thinkingTypes: ['disabled', 'adaptive', 'enabled'],
    loading: false,
  }),
}));

// Mock CommandPalette
vi.mock('./CommandPalette', () => ({
  CommandPalette: () => <div data-testid="mock-command-palette">Commands</div>,
}));

// Mock ThinkingToggle
vi.mock('./ThinkingToggle', () => ({
  ThinkingToggle: () => <div data-testid="mock-thinking-toggle">Thinking</div>,
}));

// Mock localStorage
const localStorageMock: Record<string, string> = {};
Object.defineProperty(window, 'localStorage', {
  value: {
    getItem: (key: string) => localStorageMock[key] ?? null,
    setItem: (key: string, val: string) => { localStorageMock[key] = val; },
    removeItem: (key: string) => { delete localStorageMock[key]; },
    clear: () => { for (const k of Object.keys(localStorageMock)) delete localStorageMock[k]; },
  },
  writable: true,
});

const defaultProps = () => ({
  onSend: vi.fn(),
  onStop: vi.fn(),
  disabled: false,
  isStreaming: false,
});

describe('ChatInput', () => {
  afterEach(() => {
    cleanup();
    for (const k of Object.keys(localStorageMock)) delete localStorageMock[k];
  });

  it('renders the chat input textarea', () => {
    render(<ChatInput {...defaultProps()} />);
    expect(screen.getByTestId('chat-input')).toBeTruthy();
  });

  it('renders send button', () => {
    render(<ChatInput {...defaultProps()} />);
    expect(screen.getByTestId('send-button')).toBeTruthy();
  });

  it('does not send empty messages', () => {
    const props = defaultProps();
    render(<ChatInput {...props} />);
    fireEvent.click(screen.getByTestId('send-button'));
    expect(props.onSend).not.toHaveBeenCalled();
  });

  it('sends message when text entered and send clicked', () => {
    const props = defaultProps();
    render(<ChatInput {...props} />);
    const textarea = screen.getByTestId('chat-input') as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: 'Hello world' } });
    fireEvent.click(screen.getByTestId('send-button'));
    expect(props.onSend).toHaveBeenCalledWith('Hello world', expect.any(Object));
  });

  it('clears textarea after sending', () => {
    const props = defaultProps();
    render(<ChatInput {...props} />);
    const textarea = screen.getByTestId('chat-input') as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: 'Hello world' } });
    fireEvent.click(screen.getByTestId('send-button'));
    expect(textarea.value).toBe('');
  });

  it('Enter key sends message', () => {
    const props = defaultProps();
    render(<ChatInput {...props} />);
    const textarea = screen.getByTestId('chat-input') as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: 'Test message' } });
    fireEvent.keyDown(textarea, { key: 'Enter', shiftKey: false });
    expect(props.onSend).toHaveBeenCalled();
  });

  it('Shift+Enter does not send message', () => {
    const props = defaultProps();
    render(<ChatInput {...props} />);
    const textarea = screen.getByTestId('chat-input') as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: 'Test' } });
    fireEvent.keyDown(textarea, { key: 'Enter', shiftKey: true });
    expect(props.onSend).not.toHaveBeenCalled();
  });

  it('shows stop button when streaming', () => {
    render(<ChatInput {...defaultProps()} isStreaming={true} />);
    expect(screen.getByTestId('stop-button')).toBeTruthy();
  });

  it('stop button calls onStop', () => {
    const props = defaultProps();
    props.isStreaming = true;
    render(<ChatInput {...props} />);
    fireEvent.click(screen.getByTestId('stop-button'));
    expect(props.onStop).toHaveBeenCalled();
  });

  it('does not send when disabled', () => {
    const props = defaultProps();
    props.disabled = true;
    render(<ChatInput {...props} />);
    const textarea = screen.getByTestId('chat-input') as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: 'Hello' } });
    fireEvent.click(screen.getByTestId('send-button'));
    expect(props.onSend).not.toHaveBeenCalled();
  });

  it('Escape stops streaming when isStreaming', () => {
    const props = defaultProps();
    props.isStreaming = true;
    render(<ChatInput {...props} />);
    const textarea = screen.getByTestId('chat-input') as HTMLTextAreaElement;
    fireEvent.keyDown(textarea, { key: 'Escape' });
    expect(props.onStop).toHaveBeenCalled();
  });

  it('renders product selector button', () => {
    render(<ChatInput {...defaultProps()} />);
    expect(screen.getByTestId('product-selector-button')).toBeTruthy();
  });

  it('renders chat mode selector', () => {
    render(<ChatInput {...defaultProps()} />);
    expect(screen.getByTestId('chat-mode-selector')).toBeTruthy();
  });

  it('renders attach button', () => {
    render(<ChatInput {...defaultProps()} />);
    expect(screen.getByTestId('attach-button')).toBeTruthy();
  });

  it('renders code snippet button', () => {
    render(<ChatInput {...defaultProps()} />);
    expect(screen.getByTestId('code-snippet-button')).toBeTruthy();
  });

  it('toggles code snippet input on button click', () => {
    render(<ChatInput {...defaultProps()} />);
    // Initially no code input
    expect(screen.queryByTestId('code-snippet-input')).toBeNull();
    // Click code snippet button
    fireEvent.click(screen.getByTestId('code-snippet-button'));
    expect(screen.getByTestId('code-snippet-input')).toBeTruthy();
  });

  it('renders model selector', () => {
    render(<ChatInput {...defaultProps()} />);
    expect(screen.getByTestId('model-selector')).toBeTruthy();
  });

  it('renders settings button', () => {
    render(<ChatInput {...defaultProps()} />);
    expect(screen.getByTestId('settings-btn')).toBeTruthy();
  });

  it('renders chat mode buttons (chat/code/architect/brainstorm)', () => {
    render(<ChatInput {...defaultProps()} />);
    expect(screen.getByTestId('chat-mode-chat')).toBeTruthy();
    expect(screen.getByTestId('chat-mode-code')).toBeTruthy();
    expect(screen.getByTestId('chat-mode-architect')).toBeTruthy();
    expect(screen.getByTestId('chat-mode-brainstorm')).toBeTruthy();
  });

  it('shows product badge when product is selected', () => {
    render(<ChatInput {...defaultProps()} />);
    // Click product selector to toggle product menu
    fireEvent.click(screen.getByTestId('product-selector-button'));
    // Product options should appear
    expect(screen.getByTestId('product-option-none')).toBeTruthy();
  });

  it('sends with model option when non-default model selected', () => {
    const props = defaultProps();
    render(<ChatInput {...props} />);
    const textarea = screen.getByTestId('chat-input') as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: 'Test' } });
    fireEvent.click(screen.getByTestId('send-button'));
    // onSend should be called with options object containing model
    expect(props.onSend).toHaveBeenCalledWith('Test', expect.objectContaining({
      model: expect.any(String),
    }));
  });

  it('hides send button and shows stop button during streaming', () => {
    render(<ChatInput {...defaultProps()} isStreaming={true} />);
    expect(screen.getByTestId('stop-button')).toBeTruthy();
    // Send button should not be visible (stop replaces it)
    expect(screen.queryByTestId('send-button')).toBeNull();
  });

  it('textarea remains enabled during streaming for queuing messages', () => {
    render(<ChatInput {...defaultProps()} isStreaming={true} />);
    const textarea = screen.getByTestId('chat-input') as HTMLTextAreaElement;
    // Textarea stays enabled so users can type while streaming
    expect(textarea.disabled).toBe(false);
  });

  it('renders camera button', () => {
    render(<ChatInput {...defaultProps()} />);
    expect(screen.getByTestId('camera-button')).toBeTruthy();
  });

  it('chat mode buttons switch active mode', () => {
    render(<ChatInput {...defaultProps()} />);
    const codeBtn = screen.getByTestId('chat-mode-code');
    fireEvent.click(codeBtn);
    // After clicking code, it should have active styling (font-semibold)
    expect(codeBtn.className).toContain('font-semibold');
  });

  it('placeholder changes for brainstorm mode', () => {
    render(<ChatInput {...defaultProps()} />);
    fireEvent.click(screen.getByTestId('chat-mode-brainstorm'));
    const textarea = screen.getByTestId('chat-input') as HTMLTextAreaElement;
    expect(textarea.placeholder).toContain('build');
  });

  it('placeholder changes for architect mode', () => {
    render(<ChatInput {...defaultProps()} />);
    fireEvent.click(screen.getByTestId('chat-mode-architect'));
    const textarea = screen.getByTestId('chat-input') as HTMLTextAreaElement;
    expect(textarea.placeholder).toContain('architecture');
  });

  it('placeholder changes for code mode', () => {
    render(<ChatInput {...defaultProps()} />);
    fireEvent.click(screen.getByTestId('chat-mode-code'));
    const textarea = screen.getByTestId('chat-input') as HTMLTextAreaElement;
    expect(textarea.placeholder).toContain('code');
  });

  it('default placeholder says Message...', () => {
    render(<ChatInput {...defaultProps()} />);
    const textarea = screen.getByTestId('chat-input') as HTMLTextAreaElement;
    expect(textarea.placeholder).toBe('Message...');
  });

  it('does not send whitespace-only messages', () => {
    const props = defaultProps();
    render(<ChatInput {...props} />);
    const textarea = screen.getByTestId('chat-input') as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: '   ' } });
    fireEvent.click(screen.getByTestId('send-button'));
    expect(props.onSend).not.toHaveBeenCalled();
  });

  it('Escape does not stop streaming when not streaming', () => {
    const props = defaultProps();
    render(<ChatInput {...props} />);
    const textarea = screen.getByTestId('chat-input') as HTMLTextAreaElement;
    fireEvent.keyDown(textarea, { key: 'Escape' });
    expect(props.onStop).not.toHaveBeenCalled();
  });

  it('product badge shows General when no product selected', () => {
    render(<ChatInput {...defaultProps()} />);
    expect(screen.getByTestId('product-badge').textContent).toBe('General');
  });

  it('toggles code snippet off on second click', () => {
    render(<ChatInput {...defaultProps()} />);
    // Click to open
    fireEvent.click(screen.getByTestId('code-snippet-button'));
    expect(screen.getByTestId('code-snippet-input')).toBeTruthy();
    // Click again to close
    fireEvent.click(screen.getByTestId('code-snippet-button'));
    expect(screen.queryByTestId('code-snippet-input')).toBeNull();
  });

  it('textarea is disabled when disabled prop is true', () => {
    const props = defaultProps();
    props.disabled = true;
    render(<ChatInput {...props} />);
    expect((screen.getByTestId('chat-input') as HTMLTextAreaElement).disabled).toBe(true);
  });
});
