// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ProfileGate } from './ProfileGate';

vi.mock('../../lib/api', () => ({
  api: {
    post: vi.fn().mockResolvedValue({
      score: 75,
      strengths: ['Good'],
      gaps: ['Missing'],
      recommendations: ['Add more'],
      keyword_suggestions: ['AI'],
    }),
  },
}));

describe('ProfileGate', () => {
  afterEach(() => cleanup());

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
});
