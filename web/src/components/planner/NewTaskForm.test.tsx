// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import NewTaskForm from './NewTaskForm';

describe('NewTaskForm', () => {
  afterEach(() => cleanup());

  const defaultProps = () => ({
    onClose: vi.fn(),
    onCreate: vi.fn().mockResolvedValue(undefined),
  });

  it('renders new task form', () => {
    render(<NewTaskForm {...defaultProps()} />);
    expect(screen.getByText('New Task')).toBeTruthy();
  });

  it('renders title input', () => {
    render(<NewTaskForm {...defaultProps()} />);
    expect(screen.getByTestId('new-task-title')).toBeTruthy();
  });

  it('renders description textarea', () => {
    render(<NewTaskForm {...defaultProps()} />);
    expect(screen.getByTestId('new-task-description')).toBeTruthy();
  });

  it('renders cancel and submit buttons', () => {
    render(<NewTaskForm {...defaultProps()} />);
    expect(screen.getByTestId('new-task-cancel')).toBeTruthy();
    expect(screen.getByTestId('new-task-submit')).toBeTruthy();
  });

  it('calls onClose when cancel clicked', () => {
    const props = defaultProps();
    render(<NewTaskForm {...props} />);
    fireEvent.click(screen.getByTestId('new-task-cancel'));
    expect(props.onClose).toHaveBeenCalled();
  });

  it('submit button is disabled when title is empty', () => {
    render(<NewTaskForm {...defaultProps()} />);
    expect((screen.getByTestId('new-task-submit') as HTMLButtonElement).disabled).toBe(true);
  });

  it('submit button is disabled when product is empty', () => {
    render(<NewTaskForm {...defaultProps()} />);
    fireEvent.change(screen.getByTestId('new-task-title'), { target: { value: 'My Task' } });
    // Product is empty by default (no products prop, text input)
    expect((screen.getByTestId('new-task-submit') as HTMLButtonElement).disabled).toBe(true);
  });

  it('enables submit when title and product are filled', () => {
    render(<NewTaskForm {...defaultProps()} />);
    fireEvent.change(screen.getByTestId('new-task-title'), { target: { value: 'My Task' } });
    fireEvent.change(screen.getByTestId('new-task-product-input'), { target: { value: 'chat' } });
    expect((screen.getByTestId('new-task-submit') as HTMLButtonElement).disabled).toBe(false);
  });

  it('calls onCreate with form data on submit', async () => {
    const props = defaultProps();
    render(<NewTaskForm {...props} />);

    fireEvent.change(screen.getByTestId('new-task-title'), { target: { value: 'Fix bug' } });
    fireEvent.change(screen.getByTestId('new-task-description'), { target: { value: 'Detailed description here for testing purposes of more than thirty chars' } });
    fireEvent.change(screen.getByTestId('new-task-product-input'), { target: { value: 'chat' } });

    await act(async () => {
      fireEvent.submit(screen.getByTestId('new-task-submit').closest('form')!);
    });

    expect(props.onCreate).toHaveBeenCalledWith(
      'Fix bug',
      'Detailed description here for testing purposes of more than thirty chars',
      1, // default priority
      'chat',
    );
  });

  it('shows warning for empty description', () => {
    render(<NewTaskForm {...defaultProps()} />);
    // Warning should be visible immediately since description is empty
    expect(screen.getByText(/Tasks without descriptions/)).toBeTruthy();
  });

  it('shows warning for short title', () => {
    render(<NewTaskForm {...defaultProps()} />);
    fireEvent.change(screen.getByTestId('new-task-title'), { target: { value: 'Fix' } });
    expect(screen.getByText(/Try a more specific title/)).toBeTruthy();
  });

  it('shows error for critical priority without description', () => {
    render(<NewTaskForm {...defaultProps()} />);
    // Set priority to Critical (3)
    const select = screen.getByLabelText('Priority');
    fireEvent.change(select, { target: { value: '3' } });
    expect(screen.getByText('Critical tasks require a description')).toBeTruthy();
  });

  it('disables submit when validation errors exist', () => {
    render(<NewTaskForm {...defaultProps()} />);
    fireEvent.change(screen.getByTestId('new-task-title'), { target: { value: 'My Critical Task' } });
    fireEvent.change(screen.getByTestId('new-task-product-input'), { target: { value: 'chat' } });
    // Set to critical without description
    fireEvent.change(screen.getByLabelText('Priority'), { target: { value: '3' } });
    expect((screen.getByTestId('new-task-submit') as HTMLButtonElement).disabled).toBe(true);
  });

  it('pre-fills description from template', () => {
    render(<NewTaskForm {...defaultProps()} template={{ priority: 2, description: 'From template' }} />);
    expect((screen.getByTestId('new-task-description') as HTMLTextAreaElement).value).toBe('From template');
  });

  it('pre-fills product from activeProduct', () => {
    render(<NewTaskForm {...defaultProps()} activeProduct="tutor" />);
    expect((screen.getByTestId('new-task-product-input') as HTMLInputElement).value).toBe('tutor');
  });

  it('renders product dropdown when products list provided', () => {
    render(<NewTaskForm {...defaultProps()} products={['chat', 'tasks']} />);
    expect(screen.queryByTestId('new-task-product-input')).toBeNull(); // text input hidden
    expect(screen.getByText('chat')).toBeTruthy();
    expect(screen.getByText('tasks')).toBeTruthy();
  });

  it('shows "Create" text on submit button', () => {
    render(<NewTaskForm {...defaultProps()} />);
    expect(screen.getByTestId('new-task-submit').textContent).toBe('Create');
  });
});
