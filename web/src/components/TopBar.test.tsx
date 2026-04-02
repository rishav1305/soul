// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { TopBar } from './TopBar';

vi.mock('../contexts/ChatContext', () => ({
  useChatContext: vi.fn(() => ({ status: 'connected' })),
}));

describe('TopBar', () => {
  afterEach(() => cleanup());

  it('renders top bar container', () => {
    render(<TopBar title="Soul v2" />);
    expect(screen.getByTestId('top-bar')).toBeTruthy();
  });

  it('shows title', () => {
    render(<TopBar title="Chat" />);
    expect(screen.getByTestId('top-bar-title').textContent).toBe('Chat');
  });

  it('renders status indicator', () => {
    render(<TopBar title="Test" />);
    expect(screen.getByTestId('top-bar-status')).toBeTruthy();
  });

  it('has role banner', () => {
    render(<TopBar title="Test" />);
    expect(screen.getByRole('banner')).toBeTruthy();
  });

  it('renders children', () => {
    render(<TopBar title="Test"><span data-testid="child">Action</span></TopBar>);
    expect(screen.getByTestId('child')).toBeTruthy();
  });

  it('shows connected status label', () => {
    render(<TopBar title="Test" />);
    expect(screen.getByLabelText('Connection connected')).toBeTruthy();
  });
});
