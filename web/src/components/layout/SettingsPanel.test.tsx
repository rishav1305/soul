// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import SettingsPanel from './SettingsPanel';

const defaultProps = () => ({
  onClose: vi.fn(),
  chatPosition: 'right' as const,
  setChatPosition: vi.fn(),
  tasksPosition: 'bottom' as const,
  setTasksPosition: vi.fn(),
  chatSplitPct: 50,
  setChatSplitPct: vi.fn(),
  autoInjectContext: true,
  setAutoInjectContext: vi.fn(),
  showContextChip: false,
  setShowContextChip: vi.fn(),
  toastsEnabled: true,
  setToastsEnabled: vi.fn(),
  inlineBadgesEnabled: false,
  setInlineBadgesEnabled: vi.fn(),
});

describe('SettingsPanel', () => {
  afterEach(() => cleanup());

  it('renders settings panel', () => {
    render(<SettingsPanel {...defaultProps()} />);
    expect(screen.getByTestId('settings-panel')).toBeTruthy();
  });

  it('shows Settings title', () => {
    render(<SettingsPanel {...defaultProps()} />);
    expect(screen.getByText('Settings')).toBeTruthy();
  });

  it('renders close button', () => {
    render(<SettingsPanel {...defaultProps()} />);
    expect(screen.getByTestId('settings-close-btn')).toBeTruthy();
  });

  it('calls onClose when close button clicked', () => {
    const props = defaultProps();
    render(<SettingsPanel {...props} />);
    fireEvent.click(screen.getByTestId('settings-close-btn'));
    expect(props.onClose).toHaveBeenCalled();
  });

  it('calls onClose when backdrop clicked', () => {
    const props = defaultProps();
    render(<SettingsPanel {...props} />);
    fireEvent.click(screen.getByTestId('settings-backdrop'));
    expect(props.onClose).toHaveBeenCalled();
  });

  it('renders position switches for chat and tasks', () => {
    render(<SettingsPanel {...defaultProps()} />);
    expect(screen.getByText('Panel Positions')).toBeTruthy();
    expect(screen.getByTestId('settings-position-chat-top')).toBeTruthy();
    expect(screen.getByTestId('settings-position-chat-bottom')).toBeTruthy();
    expect(screen.getByTestId('settings-position-chat-right')).toBeTruthy();
    expect(screen.getByTestId('settings-position-tasks-top')).toBeTruthy();
  });

  it('calls setChatPosition when position clicked', () => {
    const props = defaultProps();
    render(<SettingsPanel {...props} />);
    fireEvent.click(screen.getByTestId('settings-position-chat-top'));
    expect(props.setChatPosition).toHaveBeenCalledWith('top');
  });

  it('calls setTasksPosition when position clicked', () => {
    const props = defaultProps();
    render(<SettingsPanel {...props} />);
    fireEvent.click(screen.getByTestId('settings-position-tasks-right'));
    expect(props.setTasksPosition).toHaveBeenCalledWith('right');
  });

  it('renders chat split slider', () => {
    render(<SettingsPanel {...defaultProps()} />);
    expect(screen.getByTestId('settings-chat-split-slider')).toBeTruthy();
  });

  it('shows current split percentages', () => {
    render(<SettingsPanel {...defaultProps()} />);
    expect(screen.getByText('Chat 50%')).toBeTruthy();
    expect(screen.getByText('Tasks 50%')).toBeTruthy();
  });

  it('renders toggle controls', () => {
    render(<SettingsPanel {...defaultProps()} />);
    expect(screen.getByText('Auto-inject on new chat')).toBeTruthy();
    expect(screen.getByText('Show chip on product switch')).toBeTruthy();
    expect(screen.getByText('Stage change toasts')).toBeTruthy();
    expect(screen.getByText('Inline task card badges')).toBeTruthy();
  });

  it('calls setAutoInjectContext when toggle clicked', () => {
    const props = defaultProps();
    render(<SettingsPanel {...props} />);
    fireEvent.click(screen.getByTestId('settings-toggle-auto-inject-on-new-chat'));
    // Was true, so should call with false
    expect(props.setAutoInjectContext).toHaveBeenCalledWith(false);
  });

  it('calls setToastsEnabled when toggle clicked', () => {
    const props = defaultProps();
    render(<SettingsPanel {...props} />);
    fireEvent.click(screen.getByTestId('settings-toggle-stage-change-toasts'));
    expect(props.setToastsEnabled).toHaveBeenCalledWith(false);
  });

  it('shows version info', () => {
    render(<SettingsPanel {...defaultProps()} />);
    expect(screen.getByText('Soul v0.2.0-alpha')).toBeTruthy();
  });

  it('close button has aria-label', () => {
    render(<SettingsPanel {...defaultProps()} />);
    expect(screen.getByTestId('settings-close-btn').getAttribute('aria-label')).toBe('Close settings');
  });
});
