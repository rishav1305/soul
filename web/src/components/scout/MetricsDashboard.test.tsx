// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import { MetricsDashboard } from './MetricsDashboard';

const mockMetrics = {
  total_posts: 42,
  total_impressions: 15000,
  avg_engagement_rate: 3.5,
  top_performing: [
    { topic: 'AI Agents', impressions: 5000 },
    { topic: 'Go Performance', impressions: 3000 },
  ],
  analysis: 'Strong engagement on AI topics',
  recommendations: ['Post more about AI', 'Add visuals'],
};

vi.mock('../../lib/api', () => ({
  api: {
    post: vi.fn().mockResolvedValue({
      total_posts: 42,
      total_impressions: 15000,
      avg_engagement_rate: 3.5,
      top_performing: [
        { topic: 'AI Agents', impressions: 5000 },
        { topic: 'Go Performance', impressions: 3000 },
      ],
      analysis: 'Strong engagement on AI topics',
      recommendations: ['Post more about AI', 'Add visuals'],
    }),
  },
}));

describe('MetricsDashboard', () => {
  afterEach(() => cleanup());

  it('renders metrics dashboard container', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByTestId('metrics-dashboard')).toBeTruthy();
  });

  it('renders platform filter buttons', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByTestId('metrics-platform-filter')).toBeTruthy();
    expect(screen.getByTestId('metrics-filter-all')).toBeTruthy();
    expect(screen.getByTestId('metrics-filter-linkedin')).toBeTruthy();
    expect(screen.getByTestId('metrics-filter-x')).toBeTruthy();
  });

  it('renders refresh button', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByTestId('metrics-refresh-btn')).toBeTruthy();
  });

  it('shows stat cards after loading', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByTestId('metrics-stat-posts')).toBeTruthy();
    expect(screen.getByTestId('metrics-stat-impressions')).toBeTruthy();
    expect(screen.getByTestId('metrics-stat-engagement')).toBeTruthy();
  });

  it('shows total posts', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByTestId('metrics-stat-posts').textContent).toContain('42');
  });

  it('shows engagement rate', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByTestId('metrics-stat-engagement').textContent).toContain('3.5%');
  });

  it('shows top performing section', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByTestId('metrics-top-performing')).toBeTruthy();
    expect(screen.getByText('AI Agents')).toBeTruthy();
  });

  it('shows AI analysis', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByTestId('metrics-analysis')).toBeTruthy();
    expect(screen.getByText('Strong engagement on AI topics')).toBeTruthy();
  });

  it('shows recommendations', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByTestId('metrics-recommendations')).toBeTruthy();
    expect(screen.getByText('Post more about AI')).toBeTruthy();
  });

  it('calls onRefresh when refresh clicked', async () => {
    const onRefresh = vi.fn();
    await act(async () => {
      render(<MetricsDashboard onRefresh={onRefresh} />);
    });
    await act(async () => {
      fireEvent.click(screen.getByTestId('metrics-refresh-btn'));
    });
    expect(onRefresh).toHaveBeenCalled();
  });
});
