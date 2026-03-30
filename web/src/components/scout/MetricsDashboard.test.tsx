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

  it('formats impressions with K suffix', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByTestId('metrics-stat-impressions').textContent).toContain('15.0K');
  });

  it('shows both top performing posts', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByTestId('metrics-top-post-0')).toBeTruthy();
    expect(screen.getByTestId('metrics-top-post-1')).toBeTruthy();
    expect(screen.getByText('AI Agents')).toBeTruthy();
    expect(screen.getByText('Go Performance')).toBeTruthy();
  });

  it('shows impressions for top posts', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByTestId('metrics-top-post-0').textContent).toContain('5.0K impressions');
    expect(screen.getByTestId('metrics-top-post-1').textContent).toContain('3.0K impressions');
  });

  it('shows ranking numbers for top posts', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByTestId('metrics-top-post-0').textContent).toContain('#1');
    expect(screen.getByTestId('metrics-top-post-1').textContent).toContain('#2');
  });

  it('shows recommendation items', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByTestId('metrics-recommendation-0')).toBeTruthy();
    expect(screen.getByTestId('metrics-recommendation-1')).toBeTruthy();
    expect(screen.getByText('Add visuals')).toBeTruthy();
  });

  it('platform filter buttons change active state', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    const linkedinBtn = screen.getByTestId('metrics-filter-linkedin');
    await act(async () => {
      fireEvent.click(linkedinBtn);
    });
    // After clicking LinkedIn, its class should include 'bg-soul' (active)
    expect(linkedinBtn.className).toContain('bg-soul');
  });

  it('shows stat card labels', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByText('Total Posts')).toBeTruthy();
    expect(screen.getByText('Total Impressions')).toBeTruthy();
    expect(screen.getByText('Avg Engagement Rate')).toBeTruthy();
  });

  it('shows section headers', async () => {
    await act(async () => {
      render(<MetricsDashboard />);
    });
    expect(screen.getByText('Top Performing')).toBeTruthy();
    expect(screen.getByText('AI Analysis')).toBeTruthy();
    expect(screen.getByText('Recommendations')).toBeTruthy();
  });
});
