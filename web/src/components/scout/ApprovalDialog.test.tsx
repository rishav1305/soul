// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { ApprovalDialog } from './ApprovalDialog';

function makeOptimization(overrides: Record<string, unknown> = {}) {
  return {
    id: 1,
    field: 'headline',
    type: 'rewrite',
    reason: 'More impactful wording',
    current: 'Software Engineer',
    suggested: 'Senior Software Engineer | Go & AI',
    ...overrides,
  };
}

const defaultProps = () => ({
  optimization: makeOptimization(),
  onApprove: vi.fn(),
  onReject: vi.fn(),
  onClose: vi.fn(),
});

describe('ApprovalDialog', () => {
  afterEach(() => cleanup());

  it('renders approval dialog', () => {
    render(<ApprovalDialog {...defaultProps()} />);
    expect(screen.getByTestId('approval-dialog')).toBeTruthy();
  });

  it('shows Review Optimization title', () => {
    render(<ApprovalDialog {...defaultProps()} />);
    expect(screen.getByText('Review Optimization')).toBeTruthy();
  });

  it('shows optimization field', () => {
    render(<ApprovalDialog {...defaultProps()} />);
    expect(screen.getByText('headline')).toBeTruthy();
  });

  it('shows optimization type', () => {
    render(<ApprovalDialog {...defaultProps()} />);
    expect(screen.getByText('rewrite')).toBeTruthy();
  });

  it('shows optimization reason', () => {
    render(<ApprovalDialog {...defaultProps()} />);
    expect(screen.getByText('More impactful wording')).toBeTruthy();
  });

  it('shows current and suggested values', () => {
    render(<ApprovalDialog {...defaultProps()} />);
    expect(screen.getByText('Software Engineer')).toBeTruthy();
    expect(screen.getByText('Senior Software Engineer | Go & AI')).toBeTruthy();
  });

  it('shows Current and Suggested labels', () => {
    render(<ApprovalDialog {...defaultProps()} />);
    expect(screen.getByText('Current')).toBeTruthy();
    expect(screen.getByText('Suggested')).toBeTruthy();
  });

  it('calls onApprove with optimization id when Approve clicked', () => {
    const props = defaultProps();
    render(<ApprovalDialog {...props} />);
    fireEvent.click(screen.getByTestId('approval-approve'));
    expect(props.onApprove).toHaveBeenCalledWith(1);
  });

  it('calls onReject with optimization id when Reject clicked', () => {
    const props = defaultProps();
    render(<ApprovalDialog {...props} />);
    fireEvent.click(screen.getByTestId('approval-reject'));
    expect(props.onReject).toHaveBeenCalledWith(1);
  });

  it('calls onClose when Close clicked', () => {
    const props = defaultProps();
    render(<ApprovalDialog {...props} />);
    fireEvent.click(screen.getByTestId('approval-dialog-close'));
    expect(props.onClose).toHaveBeenCalled();
  });
});
