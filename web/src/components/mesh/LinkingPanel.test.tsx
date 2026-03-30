// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import { LinkingPanel } from './LinkingPanel';

describe('LinkingPanel', () => {
  afterEach(() => cleanup());

  const defaultProps = () => ({
    linkCode: null as string | null,
    onGenerateCode: vi.fn().mockResolvedValue(undefined),
    onLinkNode: vi.fn().mockResolvedValue(undefined),
  });

  it('renders linking panel', () => {
    render(<LinkingPanel {...defaultProps()} />);
    expect(screen.getByTestId('linking-panel')).toBeTruthy();
  });

  it('shows generate code button', () => {
    render(<LinkingPanel {...defaultProps()} />);
    expect(screen.getByTestId('generate-code')).toBeTruthy();
    expect(screen.getByText('Generate Code')).toBeTruthy();
  });

  it('shows Join with Code section', () => {
    render(<LinkingPanel {...defaultProps()} />);
    expect(screen.getByText('Join with Code')).toBeTruthy();
    expect(screen.getByTestId('enter-code')).toBeTruthy();
    expect(screen.getByTestId('join-btn')).toBeTruthy();
  });

  it('calls onGenerateCode when generate clicked', async () => {
    const props = defaultProps();
    render(<LinkingPanel {...props} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('generate-code'));
    });
    expect(props.onGenerateCode).toHaveBeenCalled();
  });

  it('shows pairing code when linkCode is set', () => {
    render(<LinkingPanel {...defaultProps()} linkCode="ABC-123-XYZ" />);
    expect(screen.getByTestId('pairing-code')).toBeTruthy();
    expect(screen.getByText('ABC-123-XYZ')).toBeTruthy();
  });

  it('hides pairing code when linkCode is null', () => {
    render(<LinkingPanel {...defaultProps()} />);
    expect(screen.queryByTestId('pairing-code')).toBeNull();
  });

  it('join button is disabled when code input is empty', () => {
    render(<LinkingPanel {...defaultProps()} />);
    expect((screen.getByTestId('join-btn') as HTMLButtonElement).disabled).toBe(true);
  });

  it('join button is enabled when code is entered', () => {
    render(<LinkingPanel {...defaultProps()} />);
    fireEvent.change(screen.getByTestId('enter-code'), { target: { value: 'XYZ-999' } });
    expect((screen.getByTestId('join-btn') as HTMLButtonElement).disabled).toBe(false);
  });

  it('calls onLinkNode with entered code', async () => {
    const props = defaultProps();
    render(<LinkingPanel {...props} />);
    fireEvent.change(screen.getByTestId('enter-code'), { target: { value: 'CODE-123' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('join-btn'));
    });
    expect(props.onLinkNode).toHaveBeenCalledWith('CODE-123');
  });

  it('shows status message after successful generate', async () => {
    render(<LinkingPanel {...defaultProps()} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('generate-code'));
    });
    expect(screen.getByTestId('link-status').textContent).toBe('Pairing code generated.');
  });

  it('shows success message after linking', async () => {
    render(<LinkingPanel {...defaultProps()} />);
    fireEvent.change(screen.getByTestId('enter-code'), { target: { value: 'CODE-123' } });
    await act(async () => {
      fireEvent.click(screen.getByTestId('join-btn'));
    });
    expect(screen.getByTestId('link-status').textContent).toBe('Node linked successfully.');
  });

  it('shows error message when generate fails', async () => {
    const props = defaultProps();
    props.onGenerateCode = vi.fn().mockRejectedValue(new Error('fail'));
    render(<LinkingPanel {...props} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('generate-code'));
    });
    expect(screen.getByTestId('link-status').textContent).toBe('Failed to generate code.');
  });
});
