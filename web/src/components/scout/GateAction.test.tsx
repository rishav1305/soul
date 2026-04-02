// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { GateAction } from './GateAction';

describe('GateAction', () => {
  afterEach(() => cleanup());

  it('renders action bar container', () => {
    render(<GateAction actions={[]} onAction={vi.fn()} />);
    expect(screen.getByTestId('gate-action-bar')).toBeTruthy();
  });

  it('renders action buttons', () => {
    render(<GateAction actions={['approve', 'reject']} onAction={vi.fn()} />);
    expect(screen.getByTestId('gate-action-approve')).toBeTruthy();
    expect(screen.getByTestId('gate-action-reject')).toBeTruthy();
  });

  it('shows action text as label', () => {
    render(<GateAction actions={['approve', 'send']} onAction={vi.fn()} />);
    expect(screen.getByText('approve')).toBeTruthy();
    expect(screen.getByText('send')).toBeTruthy();
  });

  it('calls onAction with action name when clicked', () => {
    const onAction = vi.fn();
    render(<GateAction actions={['approve', 'skip']} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('gate-action-approve'));
    expect(onAction).toHaveBeenCalledWith('approve');
  });

  it('shows Working... when loading', () => {
    render(<GateAction actions={['approve']} onAction={vi.fn()} loading />);
    expect(screen.getByText('Working...')).toBeTruthy();
  });

  it('disables buttons when loading', () => {
    render(<GateAction actions={['approve']} onAction={vi.fn()} loading />);
    expect((screen.getByTestId('gate-action-approve') as HTMLButtonElement).disabled).toBe(true);
  });

  it('disables buttons when disabled prop is true', () => {
    render(<GateAction actions={['approve']} onAction={vi.fn()} disabled />);
    expect((screen.getByTestId('gate-action-approve') as HTMLButtonElement).disabled).toBe(true);
  });

  it('renders all provided actions', () => {
    const actions = ['approve', 'edit', 'skip', 'send', 'reject'];
    render(<GateAction actions={actions} onAction={vi.fn()} />);
    for (const a of actions) {
      expect(screen.getByTestId(`gate-action-${a}`)).toBeTruthy();
    }
  });
});
