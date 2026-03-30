// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ProfilePanel } from './ProfilePanel';

function makeProfile(overrides: Record<string, unknown> = {}) {
  return {
    experience: [
      { title: 'Senior Engineer', company: 'Acme', duration: '2020-2024', description: 'Built systems' },
    ],
    projects: [
      { name: 'Soul v2', description: 'AI chat interface', url: 'https://example.com' },
    ],
    skills: ['Go', 'React', 'TypeScript'],
    education: [
      { degree: 'B.Tech CS', institution: 'IIT', year: '2016' },
    ],
    certifications: [
      { name: 'AWS SAA', issuer: 'Amazon', year: '2023' },
    ],
    ...overrides,
  } as any;
}

describe('ProfilePanel', () => {
  afterEach(() => cleanup());

  it('renders profile panel container', () => {
    render(<ProfilePanel profile={makeProfile()} />);
    expect(screen.getByTestId('profile-panel')).toBeTruthy();
  });

  it('renders all 5 section toggles', () => {
    render(<ProfilePanel profile={makeProfile()} />);
    expect(screen.getByTestId('profile-section-experience')).toBeTruthy();
    expect(screen.getByTestId('profile-section-projects')).toBeTruthy();
    expect(screen.getByTestId('profile-section-skills')).toBeTruthy();
    expect(screen.getByTestId('profile-section-education')).toBeTruthy();
    expect(screen.getByTestId('profile-section-certifications')).toBeTruthy();
  });

  it('shows section counts', () => {
    render(<ProfilePanel profile={makeProfile()} />);
    expect(screen.getByTestId('profile-section-experience').textContent).toContain('Experience (1)');
    expect(screen.getByTestId('profile-section-projects').textContent).toContain('Projects (1)');
    expect(screen.getByTestId('profile-section-skills').textContent).toContain('Skills (3)');
    expect(screen.getByTestId('profile-section-education').textContent).toContain('Education (1)');
    expect(screen.getByTestId('profile-section-certifications').textContent).toContain('Certifications (1)');
  });

  it('experience section expanded by default', () => {
    render(<ProfilePanel profile={makeProfile()} />);
    expect(screen.getByText('Senior Engineer')).toBeTruthy();
    expect(screen.getByText('Acme - 2020-2024')).toBeTruthy();
  });

  it('collapses experience on toggle', () => {
    render(<ProfilePanel profile={makeProfile()} />);
    fireEvent.click(screen.getByTestId('profile-section-experience'));
    expect(screen.queryByText('Senior Engineer')).toBeNull();
  });

  it('expands projects on toggle', () => {
    render(<ProfilePanel profile={makeProfile()} />);
    fireEvent.click(screen.getByTestId('profile-section-projects'));
    expect(screen.getByText('Soul v2')).toBeTruthy();
    expect(screen.getByText('AI chat interface')).toBeTruthy();
  });

  it('expands skills on toggle', () => {
    render(<ProfilePanel profile={makeProfile()} />);
    fireEvent.click(screen.getByTestId('profile-section-skills'));
    expect(screen.getByText('Go')).toBeTruthy();
    expect(screen.getByText('React')).toBeTruthy();
    expect(screen.getByText('TypeScript')).toBeTruthy();
  });

  it('expands education on toggle', () => {
    render(<ProfilePanel profile={makeProfile()} />);
    fireEvent.click(screen.getByTestId('profile-section-education'));
    expect(screen.getByText('B.Tech CS')).toBeTruthy();
    expect(screen.getByText('IIT - 2016')).toBeTruthy();
  });

  it('expands certifications on toggle', () => {
    render(<ProfilePanel profile={makeProfile()} />);
    fireEvent.click(screen.getByTestId('profile-section-certifications'));
    expect(screen.getByText('AWS SAA')).toBeTruthy();
    expect(screen.getByText('Amazon - 2023')).toBeTruthy();
  });

  it('shows project URL when present', () => {
    render(<ProfilePanel profile={makeProfile()} />);
    fireEvent.click(screen.getByTestId('profile-section-projects'));
    expect(screen.getByText('https://example.com')).toBeTruthy();
  });
});
