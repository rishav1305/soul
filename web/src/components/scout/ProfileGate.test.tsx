// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act, waitFor } from '@testing-library/react';
import { ProfileGate } from './ProfileGate';

const mockPost = vi.fn().mockResolvedValue({
  score: 75,
  strengths: ['Good'],
  gaps: ['Missing'],
  recommendations: ['Add more'],
  keyword_suggestions: ['AI'],
});

vi.mock('../../lib/api', () => ({
  api: {
    post: (...args: unknown[]) => mockPost(...args),
  },
}));

describe('ProfileGate', () => {
  afterEach(() => {
    cleanup();
    mockPost.mockClear();
  });

  it('renders profile gate container', () => {
    render(<ProfileGate />);
    expect(screen.getByTestId('profile-gate')).toBeTruthy();
  });

  it('renders all 5 section toggles', () => {
    render(<ProfileGate />);
    expect(screen.getByTestId('profile-section-audit')).toBeTruthy();
    expect(screen.getByTestId('profile-section-linkedin')).toBeTruthy();
    expect(screen.getByTestId('profile-section-github')).toBeTruthy();
    expect(screen.getByTestId('profile-section-testimonial')).toBeTruthy();
    expect(screen.getByTestId('profile-section-pin')).toBeTruthy();
  });

  it('shows section labels', () => {
    render(<ProfileGate />);
    expect(screen.getByTestId('profile-section-audit').textContent).toContain('Profile Audit');
    expect(screen.getByTestId('profile-section-linkedin').textContent).toContain('LinkedIn Update');
    expect(screen.getByTestId('profile-section-github').textContent).toContain('GitHub README');
    expect(screen.getByTestId('profile-section-testimonial').textContent).toContain('Testimonial Request');
    expect(screen.getByTestId('profile-section-pin').textContent).toContain('Pin Recommendation');
  });

  it('audit section expanded by default', () => {
    render(<ProfileGate />);
    expect(screen.getByTestId('profile-audit-platform')).toBeTruthy();
    expect(screen.getByTestId('profile-audit-text')).toBeTruthy();
    expect(screen.getByTestId('profile-audit-btn')).toBeTruthy();
  });

  it('audit button disabled when text empty', () => {
    render(<ProfileGate />);
    expect((screen.getByTestId('profile-audit-btn') as HTMLButtonElement).disabled).toBe(true);
  });

  it('audit button enabled when text has content', () => {
    render(<ProfileGate />);
    fireEvent.change(screen.getByTestId('profile-audit-text'), { target: { value: 'My profile' } });
    expect((screen.getByTestId('profile-audit-btn') as HTMLButtonElement).disabled).toBe(false);
  });

  it('collapses audit section on toggle', () => {
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-audit'));
    expect(screen.queryByTestId('profile-audit-btn')).toBeNull();
  });

  it('expands linkedin section on toggle', () => {
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-linkedin'));
    expect(screen.getByTestId('profile-linkedin-section')).toBeTruthy();
    expect(screen.getByTestId('profile-linkedin-content')).toBeTruthy();
    expect(screen.getByTestId('profile-linkedin-btn')).toBeTruthy();
  });

  it('expands github section on toggle', () => {
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-github'));
    expect(screen.getByTestId('profile-github-repo-name')).toBeTruthy();
    expect(screen.getByTestId('profile-github-description')).toBeTruthy();
    expect(screen.getByTestId('profile-github-btn')).toBeTruthy();
  });

  it('expands testimonial section on toggle', () => {
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-testimonial'));
    expect(screen.getByTestId('profile-testimonial-lead-id')).toBeTruthy();
    expect(screen.getByTestId('profile-testimonial-btn')).toBeTruthy();
  });

  it('expands pin section on toggle', () => {
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-pin'));
    expect(screen.getByTestId('profile-pin-platform')).toBeTruthy();
    expect(screen.getByTestId('profile-pin-btn')).toBeTruthy();
  });

  it('shows platform select with LinkedIn/GitHub options', () => {
    render(<ProfileGate />);
    const select = screen.getByTestId('profile-audit-platform') as HTMLSelectElement;
    expect(select.value).toBe('linkedin');
  });

  it('audit API call sends correct params', async () => {
    render(<ProfileGate />);
    fireEvent.change(screen.getByTestId('profile-audit-text'), { target: { value: 'My LinkedIn profile' } });
    fireEvent.change(screen.getByTestId('profile-audit-platform'), { target: { value: 'github' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-audit-btn'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/profile-audit', {
      platform: 'github',
      current_profile: 'My LinkedIn profile',
    });
  });

  it('displays audit result with score, strengths, gaps, recommendations, keywords', async () => {
    mockPost.mockResolvedValueOnce({
      score: 85,
      strengths: ['Strong headline'],
      gaps: ['Missing certifications'],
      recommendations: ['Add more projects'],
      keyword_suggestions: ['React', 'AI'],
    });
    render(<ProfileGate />);
    fireEvent.change(screen.getByTestId('profile-audit-text'), { target: { value: 'My profile' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-audit-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('profile-audit-result')).toBeTruthy();
    });
    expect(screen.getByText('85')).toBeTruthy();
    expect(screen.getByText('Strong headline')).toBeTruthy();
    expect(screen.getByText('Missing certifications')).toBeTruthy();
    expect(screen.getByText('Add more projects')).toBeTruthy();
    expect(screen.getByText('React')).toBeTruthy();
    expect(screen.getByText('AI')).toBeTruthy();
  });

  it('displays "Strong profile" for score >= 80', async () => {
    mockPost.mockResolvedValueOnce({
      score: 90, strengths: [], gaps: [], recommendations: [], keyword_suggestions: [],
    });
    render(<ProfileGate />);
    fireEvent.change(screen.getByTestId('profile-audit-text'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-audit-btn'));
    });
    await waitFor(() => expect(screen.getByText('Strong profile')).toBeTruthy());
  });

  it('displays "Needs improvement" for score 50-79', async () => {
    mockPost.mockResolvedValueOnce({
      score: 60, strengths: [], gaps: [], recommendations: [], keyword_suggestions: [],
    });
    render(<ProfileGate />);
    fireEvent.change(screen.getByTestId('profile-audit-text'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-audit-btn'));
    });
    await waitFor(() => expect(screen.getByText('Needs improvement')).toBeTruthy());
  });

  it('displays "Major gaps" for score < 50', async () => {
    mockPost.mockResolvedValueOnce({
      score: 30, strengths: [], gaps: [], recommendations: [], keyword_suggestions: [],
    });
    render(<ProfileGate />);
    fireEvent.change(screen.getByTestId('profile-audit-text'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-audit-btn'));
    });
    await waitFor(() => expect(screen.getByText('Major gaps')).toBeTruthy());
  });

  it('shows error when API call fails', async () => {
    mockPost.mockRejectedValueOnce(new Error('Network error'));
    render(<ProfileGate />);
    fireEvent.change(screen.getByTestId('profile-audit-text'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-audit-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('profile-gate-error')).toBeTruthy();
      expect(screen.getByText('Network error')).toBeTruthy();
    });
  });

  it('linkedin button disabled when content empty', () => {
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-linkedin'));
    expect((screen.getByTestId('profile-linkedin-btn') as HTMLButtonElement).disabled).toBe(true);
  });

  it('linkedin API call sends correct params', async () => {
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-linkedin'));
    fireEvent.change(screen.getByTestId('profile-linkedin-section'), { target: { value: 'about' } });
    fireEvent.change(screen.getByTestId('profile-linkedin-content'), { target: { value: 'My about section' } });
    mockPost.mockResolvedValueOnce({ updated_text: 'Improved about' });
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-linkedin-btn'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/linkedin-update', {
      section: 'about',
      current_content: 'My about section',
    });
  });

  it('displays linkedin result', async () => {
    mockPost.mockResolvedValueOnce({ updated_text: 'New headline text' });
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-linkedin'));
    fireEvent.change(screen.getByTestId('profile-linkedin-content'), { target: { value: 'Old text' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-linkedin-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('profile-linkedin-result')).toBeTruthy();
      expect(screen.getByText('New headline text')).toBeTruthy();
    });
  });

  it('github button disabled when repo name or description empty', () => {
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-github'));
    expect((screen.getByTestId('profile-github-btn') as HTMLButtonElement).disabled).toBe(true);
    // Only repo name
    fireEvent.change(screen.getByTestId('profile-github-repo-name'), { target: { value: 'soul-v2' } });
    expect((screen.getByTestId('profile-github-btn') as HTMLButtonElement).disabled).toBe(true);
  });

  it('github API call sends correct params', async () => {
    mockPost.mockResolvedValueOnce({ markdown: '# README' });
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-github'));
    fireEvent.change(screen.getByTestId('profile-github-repo-name'), { target: { value: 'soul-v2' } });
    fireEvent.change(screen.getByTestId('profile-github-description'), { target: { value: 'AI chat' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-github-btn'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/github-readme', {
      repo_name: 'soul-v2',
      description: 'AI chat',
    });
  });

  it('displays github README result', async () => {
    mockPost.mockResolvedValueOnce({ markdown: '# Soul v2\nAI chat interface' });
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-github'));
    fireEvent.change(screen.getByTestId('profile-github-repo-name'), { target: { value: 'soul-v2' } });
    fireEvent.change(screen.getByTestId('profile-github-description'), { target: { value: 'desc' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-github-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('profile-github-result')).toBeTruthy();
    });
  });

  it('testimonial button disabled when lead ID is not a number', () => {
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-testimonial'));
    expect((screen.getByTestId('profile-testimonial-btn') as HTMLButtonElement).disabled).toBe(true);
  });

  it('testimonial API call sends correct params', async () => {
    mockPost.mockResolvedValueOnce({ message: 'Please leave a review' });
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-testimonial'));
    fireEvent.change(screen.getByTestId('profile-testimonial-lead-id'), { target: { value: '42' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-testimonial-btn'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/testimonial-request', {
      lead_id: 42,
    });
  });

  it('displays testimonial result', async () => {
    mockPost.mockResolvedValueOnce({ message: 'Dear client, I would appreciate...' });
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-testimonial'));
    fireEvent.change(screen.getByTestId('profile-testimonial-lead-id'), { target: { value: '10' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-testimonial-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('profile-testimonial-result')).toBeTruthy();
    });
  });

  it('pin API call sends correct platform', async () => {
    mockPost.mockResolvedValueOnce({ recommended_pins: ['Post A', 'Post B'] });
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-pin'));
    fireEvent.change(screen.getByTestId('profile-pin-platform'), { target: { value: 'github' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-pin-btn'));
    });
    expect(mockPost).toHaveBeenCalledWith('/api/ai/pin-recommendation', {
      platform: 'github',
    });
  });

  it('displays pin recommendation results', async () => {
    mockPost.mockResolvedValueOnce({ recommended_pins: ['Pin this project', 'Pin that article'] });
    render(<ProfileGate />);
    fireEvent.click(screen.getByTestId('profile-section-pin'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-pin-btn'));
    });
    await waitFor(() => {
      expect(screen.getByTestId('profile-pin-result')).toBeTruthy();
      expect(screen.getByTestId('profile-pin-item-0')).toBeTruthy();
      expect(screen.getByTestId('profile-pin-item-1')).toBeTruthy();
      expect(screen.getByText('Pin this project')).toBeTruthy();
    });
  });

  it('multiple sections can be expanded simultaneously', () => {
    render(<ProfileGate />);
    // Audit is already expanded
    expect(screen.getByTestId('profile-audit-btn')).toBeTruthy();
    // Expand linkedin
    fireEvent.click(screen.getByTestId('profile-section-linkedin'));
    expect(screen.getByTestId('profile-linkedin-btn')).toBeTruthy();
    // Audit should still be expanded
    expect(screen.getByTestId('profile-audit-btn')).toBeTruthy();
  });

  it('shows Generating... text during loading', async () => {
    let resolvePost: (val: unknown) => void;
    mockPost.mockImplementationOnce(() => new Promise(r => { resolvePost = r; }));
    render(<ProfileGate />);
    fireEvent.change(screen.getByTestId('profile-audit-text'), { target: { value: 'test' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('profile-audit-btn'));
    });
    expect(screen.getByText('Generating...')).toBeTruthy();
    await act(async () => {
      resolvePost!({ score: 50, strengths: [], gaps: [], recommendations: [], keyword_suggestions: [] });
    });
  });
});
