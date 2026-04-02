// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { SyncStatus } from './SyncStatus';

describe('SyncStatus', () => {
  afterEach(() => cleanup());

  it('renders sync status container', () => {
    render(<SyncStatus onSync={vi.fn()} />);
    expect(screen.getByTestId('sync-status')).toBeTruthy();
  });

  it('shows Platform Sync title', () => {
    render(<SyncStatus onSync={vi.fn()} />);
    expect(screen.getByText('Platform Sync')).toBeTruthy();
  });

  it('renders all 4 platform entries', () => {
    render(<SyncStatus onSync={vi.fn()} />);
    expect(screen.getByTestId('sync-platform-linkedin')).toBeTruthy();
    expect(screen.getByTestId('sync-platform-github')).toBeTruthy();
    expect(screen.getByTestId('sync-platform-naukri')).toBeTruthy();
    expect(screen.getByTestId('sync-platform-wellfound')).toBeTruthy();
  });

  it('shows platform names', () => {
    render(<SyncStatus onSync={vi.fn()} />);
    expect(screen.getByText('LinkedIn')).toBeTruthy();
    expect(screen.getByText('GitHub')).toBeTruthy();
    expect(screen.getByText('Naukri')).toBeTruthy();
    expect(screen.getByText('Wellfound')).toBeTruthy();
  });

  it('shows sync buttons for each platform', () => {
    render(<SyncStatus onSync={vi.fn()} />);
    expect(screen.getByTestId('sync-btn-linkedin')).toBeTruthy();
    expect(screen.getByTestId('sync-btn-github')).toBeTruthy();
    expect(screen.getByTestId('sync-btn-naukri')).toBeTruthy();
    expect(screen.getByTestId('sync-btn-wellfound')).toBeTruthy();
  });

  it('calls onSync with lowercase platform name when sync clicked', () => {
    const onSync = vi.fn();
    render(<SyncStatus onSync={onSync} />);
    fireEvent.click(screen.getByTestId('sync-btn-linkedin'));
    expect(onSync).toHaveBeenCalledWith('linkedin');
  });

  it('calls onSync with github when github sync clicked', () => {
    const onSync = vi.fn();
    render(<SyncStatus onSync={onSync} />);
    fireEvent.click(screen.getByTestId('sync-btn-github'));
    expect(onSync).toHaveBeenCalledWith('github');
  });
});
