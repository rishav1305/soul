// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act, waitFor } from '@testing-library/react';
import { ContentGate } from './ContentGate';

const mockPost = vi.fn().mockResolvedValue({ topics: [] });

vi.mock('../../lib/api', () => ({
  api: {
    post: (...args: unknown[]) => mockPost(...args),
  },
}));

describe('ContentGate', () => {
  afterEach(() => {
    cleanup();
    mockPost.mockClear();
  });

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

  it('generate topics calls API with week_summary', async () => {
    mockPost.mockResolvedValueOnce({ topics: [] });
    render(<ContentGate />);
    fireEvent.change(screen.getByTestId('content-week-summary'), { target: { value: 'Built ML pipeline' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('content-generate-topics-btn'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/content-topic', {
      week_summary: 'Built ML pipeline',
    });
  });

  it('displays generated topic cards', async () => {
    mockPost.mockResolvedValueOnce({
      topics: [
        { topic: 'AI in Production', angle: 'From prototype to production', audience: 'Engineers' },
        { topic: 'ML Pipeline Design', angle: 'Best practices', audience: 'Data Scientists' },
      ],
    });
    render(<ContentGate />);
    fireEvent.change(screen.getByTestId('content-week-summary'), { target: { value: 'Built ML pipeline' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('content-generate-topics-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('content-topic-card-0')).toBeTruthy();
      expect(screen.getByTestId('content-topic-card-1')).toBeTruthy();
    });
    expect(screen.getByText('AI in Production')).toBeTruthy();
    expect(screen.getByText('From prototype to production')).toBeTruthy();
    expect(screen.getByText('Audience: Engineers')).toBeTruthy();
  });

  it('topic card has Create Series button', async () => {
    mockPost.mockResolvedValueOnce({
      topics: [{ topic: 'AI', angle: 'Production', audience: 'Devs' }],
    });
    render(<ContentGate />);
    fireEvent.change(screen.getByTestId('content-week-summary'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('content-generate-topics-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('content-create-series-btn-0')).toBeTruthy();
    });
  });

  it('shows error when API call fails', async () => {
    mockPost.mockRejectedValueOnce(new Error('API unavailable'));
    render(<ContentGate />);
    fireEvent.change(screen.getByTestId('content-week-summary'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('content-generate-topics-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('content-gate-error')).toBeTruthy();
      expect(screen.getByText('API unavailable')).toBeTruthy();
    });
  });

  it('shows Generating... during loading', async () => {
    let resolvePost: (val: unknown) => void;
    mockPost.mockImplementationOnce(() => new Promise(r => { resolvePost = r; }));
    render(<ContentGate />);
    fireEvent.change(screen.getByTestId('content-week-summary'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('content-generate-topics-btn'));
    });
    expect(screen.getByText('Generating...')).toBeTruthy();
    await act(async () => {
      resolvePost!({ topics: [] });
    });
  });

  it('does not call onAction when onAction not provided', () => {
    render(<ContentGate />);
    // Clicking buttons should not throw when onAction is undefined
    fireEvent.click(screen.getByTestId('content-publish-btn'));
    fireEvent.click(screen.getByTestId('content-schedule-btn'));
    fireEvent.click(screen.getByTestId('content-save-draft-btn'));
  });

  it('shows section labels with Generate Topics', () => {
    render(<ContentGate />);
    const matches = screen.getAllByText('Generate Topics');
    expect(matches.length).toBeGreaterThanOrEqual(1);
  });

  it('topic input renders as optional field', () => {
    render(<ContentGate />);
    const input = screen.getByTestId('content-topic-input') as HTMLInputElement;
    expect(input.placeholder).toContain('Specific topic');
  });
});
