// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import ChatRail from './ChatRail';

describe('ChatRail', () => {
  afterEach(() => cleanup());

  it('renders chat rail button', () => {
    render(<ChatRail unreadCount={0} onExpand={vi.fn()} />);
    expect(screen.getByTestId('chat-rail')).toBeTruthy();
  });

  it('has aria-label for accessibility', () => {
    render(<ChatRail unreadCount={0} onExpand={vi.fn()} />);
    expect(screen.getByTestId('chat-rail').getAttribute('aria-label')).toBe('Open chat panel');
  });

  it('calls onExpand when clicked', () => {
    const onExpand = vi.fn();
    render(<ChatRail unreadCount={0} onExpand={onExpand} />);
    fireEvent.click(screen.getByTestId('chat-rail'));
    expect(onExpand).toHaveBeenCalled();
  });

  it('hides unread badge when count is 0', () => {
    render(<ChatRail unreadCount={0} onExpand={vi.fn()} />);
    expect(screen.queryByTestId('chat-rail-unread-badge')).toBeNull();
  });

  it('shows unread badge when count > 0', () => {
    render(<ChatRail unreadCount={3} onExpand={vi.fn()} />);
    expect(screen.getByTestId('chat-rail-unread-badge')).toBeTruthy();
    expect(screen.getByText('3')).toBeTruthy();
  });

  it('shows 9+ for unread count > 9', () => {
    render(<ChatRail unreadCount={15} onExpand={vi.fn()} />);
    expect(screen.getByText('9+')).toBeTruthy();
  });

  it('shows exact count for unread <= 9', () => {
    render(<ChatRail unreadCount={9} onExpand={vi.fn()} />);
    expect(screen.getByText('9')).toBeTruthy();
  });

  it('renders expand button', () => {
    render(<ChatRail unreadCount={0} onExpand={vi.fn()} />);
    expect(screen.getByTestId('chat-rail-expand-btn')).toBeTruthy();
  });

  it('renders as button element', () => {
    render(<ChatRail unreadCount={0} onExpand={vi.fn()} />);
    expect(screen.getByTestId('chat-rail').tagName).toBe('BUTTON');
  });
});
