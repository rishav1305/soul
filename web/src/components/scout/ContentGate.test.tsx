// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ContentGate } from './ContentGate';

vi.mock('../../lib/api', () => ({
  api: {
    post: vi.fn().mockResolvedValue({ topics: [] }),
  },
}));

describe('ContentGate', () => {
  afterEach(() => cleanup());

  it('renders content gate container', () => {
    render(<ContentGate />);
    expect(screen.getByTestId('content-gate')).toBeTruthy();
  });

  it('renders topics section toggle', () => {
    render(<ContentGate />);
    expect(screen.getByTestId('content-section-topics')).toBeTruthy();
  });

  it('topics section expanded by default', () => {
    render(<ContentGate />);
    expect(screen.getByTestId('content-week-summary')).toBeTruthy();
    expect(screen.getByTestId('content-topic-input')).toBeTruthy();
    expect(screen.getByTestId('content-generate-topics-btn')).toBeTruthy();
  });

  it('shows Week Summary label', () => {
    render(<ContentGate />);
    expect(screen.getByText('Week Summary')).toBeTruthy();
  });

  it('generate button disabled when summary empty', () => {
    render(<ContentGate />);
    expect((screen.getByTestId('content-generate-topics-btn') as HTMLButtonElement).disabled).toBe(true);
  });

  it('generate button enabled when summary has text', () => {
    render(<ContentGate />);
    fireEvent.change(screen.getByTestId('content-week-summary'), { target: { value: 'Built ML pipeline' } });
    expect((screen.getByTestId('content-generate-topics-btn') as HTMLButtonElement).disabled).toBe(false);
  });

  it('collapses topics section on toggle', () => {
    render(<ContentGate />);
    fireEvent.click(screen.getByTestId('content-section-topics'));
    expect(screen.queryByTestId('content-week-summary')).toBeNull();
  });

  it('renders footer action buttons', () => {
    render(<ContentGate />);
    expect(screen.getByTestId('content-publish-btn')).toBeTruthy();
    expect(screen.getByTestId('content-schedule-btn')).toBeTruthy();
    expect(screen.getByTestId('content-save-draft-btn')).toBeTruthy();
  });

  it('calls onAction with publish', () => {
    const onAction = vi.fn();
    render(<ContentGate onAction={onAction} />);
    fireEvent.click(screen.getByTestId('content-publish-btn'));
    expect(onAction).toHaveBeenCalledWith('publish');
  });

  it('calls onAction with schedule', () => {
    const onAction = vi.fn();
    render(<ContentGate onAction={onAction} />);
    fireEvent.click(screen.getByTestId('content-schedule-btn'));
    expect(onAction).toHaveBeenCalledWith('schedule');
  });

  it('calls onAction with save-draft', () => {
    const onAction = vi.fn();
    render(<ContentGate onAction={onAction} />);
    fireEvent.click(screen.getByTestId('content-save-draft-btn'));
    expect(onAction).toHaveBeenCalledWith('save-draft');
  });

  it('shows button labels', () => {
    render(<ContentGate />);
    expect(screen.getByText('Publish')).toBeTruthy();
    expect(screen.getByText('Schedule')).toBeTruthy();
    expect(screen.getByText('Save Draft')).toBeTruthy();
  });
});
