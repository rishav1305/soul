// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { SessionsPanel } from './SessionsPanel';
import type { Session } from '../lib/types';

// Mock SessionList to isolate SessionsPanel logic
vi.mock('../components/SessionList', () => ({
  SessionList: () => <div data-testid="mock-session-list">SessionList</div>,
}));

// Mock window.matchMedia (jsdom doesn't implement it)
Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: vi.fn().mockImplementation((query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: vi.fn(),
    removeListener: vi.fn(),
    addEventListener: vi.fn(),
    removeEventListener: vi.fn(),
    dispatchEvent: vi.fn(),
  })),
});

function makeSession(overrides: Partial<Session> = {}): Session {
  return {
    id: 's1',
    title: 'Test Session',
    status: 'idle',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    messageCount: 5,
    lastMessage: 'Hello',
    unreadCount: 0,
    product: '',
    ...overrides,
  };
}

describe('SessionsPanel', () => {
  afterEach(() => cleanup());

  const defaultProps = () => ({
    open: true,
    sessions: [makeSession()],
    activeSessionID: 's1',
    onSwitch: vi.fn(),
    onDelete: vi.fn(),
    onRename: vi.fn(),
    onClose: vi.fn(),
  });

  it('renders sessions panel when open (desktop)', () => {
    // Set desktop width
    Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
    render(<SessionsPanel {...defaultProps()} />);
    expect(screen.getByTestId('sessions-panel')).toBeTruthy();
  });

  it('shows header with "Sessions" title', () => {
    Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
    render(<SessionsPanel {...defaultProps()} />);
    expect(screen.getByTestId('sessions-panel-header')).toBeTruthy();
    expect(screen.getByText('Sessions')).toBeTruthy();
  });

  it('renders close button', () => {
    Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
    render(<SessionsPanel {...defaultProps()} />);
    expect(screen.getByTestId('sessions-panel-close')).toBeTruthy();
  });

  it('calls onClose when close button clicked', () => {
    Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
    const props = defaultProps();
    render(<SessionsPanel {...props} />);
    fireEvent.click(screen.getByTestId('sessions-panel-close'));
    expect(props.onClose).toHaveBeenCalled();
  });

  it('close button has aria-label', () => {
    Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
    render(<SessionsPanel {...defaultProps()} />);
    expect(screen.getByTestId('sessions-panel-close').getAttribute('aria-label')).toBe('Close sessions panel');
  });

  it('renders SessionList child', () => {
    Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
    render(<SessionsPanel {...defaultProps()} />);
    expect(screen.getByTestId('mock-session-list')).toBeTruthy();
  });

  it('desktop panel has aria role and label', () => {
    Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
    render(<SessionsPanel {...defaultProps()} />);
    const panel = screen.getByTestId('sessions-panel');
    expect(panel.getAttribute('role')).toBe('complementary');
    expect(panel.getAttribute('aria-label')).toBe('Session history');
  });

  it('desktop panel has width 220 when open', () => {
    Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
    render(<SessionsPanel {...defaultProps()} />);
    const panel = screen.getByTestId('sessions-panel');
    expect(panel.style.width).toBe('220px');
  });

  it('desktop panel has width 0 when closed', () => {
    Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
    const props = defaultProps();
    props.open = false;
    render(<SessionsPanel {...props} />);
    const panel = screen.getByTestId('sessions-panel');
    expect(panel.style.width).toBe('0px');
  });

  it('mobile renders fixed overlay when open', () => {
    Object.defineProperty(window, 'innerWidth', { value: 400, configurable: true });
    // Override matchMedia to report mobile
    (window.matchMedia as ReturnType<typeof vi.fn>).mockImplementation((query: string) => ({
      matches: query === '(max-width: 639px)',
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
    render(<SessionsPanel {...defaultProps()} />);
    expect(screen.getByTestId('sessions-panel')).toBeTruthy();
    expect(screen.getByTestId('sessions-backdrop')).toBeTruthy();
  });

  it('mobile backdrop click calls onClose', () => {
    Object.defineProperty(window, 'innerWidth', { value: 400, configurable: true });
    (window.matchMedia as ReturnType<typeof vi.fn>).mockImplementation((query: string) => ({
      matches: query === '(max-width: 639px)',
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
    const props = defaultProps();
    render(<SessionsPanel {...props} />);
    fireEvent.click(screen.getByTestId('sessions-backdrop'));
    expect(props.onClose).toHaveBeenCalled();
  });

  it('mobile returns null when closed', () => {
    Object.defineProperty(window, 'innerWidth', { value: 400, configurable: true });
    (window.matchMedia as ReturnType<typeof vi.fn>).mockImplementation((query: string) => ({
      matches: query === '(max-width: 639px)',
      media: query,
      onchange: null,
      addListener: vi.fn(),
      removeListener: vi.fn(),
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
      dispatchEvent: vi.fn(),
    }));
    const props = defaultProps();
    props.open = false;
    const { container } = render(<SessionsPanel {...props} />);
    expect(container.innerHTML).toBe('');
  });

  it('desktop panel still renders content when closed', () => {
    Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
    const props = defaultProps();
    props.open = false;
    render(<SessionsPanel {...props} />);
    // Desktop panel exists (just width=0)
    expect(screen.getByTestId('sessions-panel')).toBeTruthy();
    expect(screen.getByTestId('mock-session-list')).toBeTruthy();
  });

  it('desktop panel has border-l styling', () => {
    Object.defineProperty(window, 'innerWidth', { value: 1024, configurable: true });
    render(<SessionsPanel {...defaultProps()} />);
    const panel = screen.getByTestId('sessions-panel');
    expect(panel.className).toContain('border-l');
  });
});
