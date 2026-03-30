// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import SoulRail from './SoulRail';

describe('SoulRail', () => {
  afterEach(() => cleanup());

  it('renders soul rail container', () => {
    render(<SoulRail onExpand={vi.fn()} />);
    expect(screen.getByTestId('soul-rail')).toBeTruthy();
  });

  it('renders chat button', () => {
    render(<SoulRail onExpand={vi.fn()} />);
    expect(screen.getByTestId('soul-rail-chat-btn')).toBeTruthy();
  });

  it('renders tasks button', () => {
    render(<SoulRail onExpand={vi.fn()} />);
    expect(screen.getByTestId('soul-rail-tasks-btn')).toBeTruthy();
  });

  it('renders settings button', () => {
    render(<SoulRail onExpand={vi.fn()} />);
    expect(screen.getByTestId('soul-rail-settings-btn')).toBeTruthy();
  });

  it('renders expand button', () => {
    render(<SoulRail onExpand={vi.fn()} />);
    expect(screen.getByTestId('soul-rail-expand-btn')).toBeTruthy();
  });

  it('calls onExpand when expand button clicked', () => {
    const onExpand = vi.fn();
    render(<SoulRail onExpand={onExpand} />);
    fireEvent.click(screen.getByTestId('soul-rail-expand-btn'));
    expect(onExpand).toHaveBeenCalled();
  });

  it('has correct titles on buttons', () => {
    render(<SoulRail onExpand={vi.fn()} />);
    expect(screen.getByTitle('Chat')).toBeTruthy();
    expect(screen.getByTitle('Tasks')).toBeTruthy();
    expect(screen.getByTitle('Settings')).toBeTruthy();
    expect(screen.getByTitle('Expand sidebar')).toBeTruthy();
  });

  it('renders soul logo diamond', () => {
    const { container } = render(<SoulRail onExpand={vi.fn()} />);
    // The diamond character ◆ (&#9670;)
    expect(container.textContent).toContain('\u25C6');
  });
});
