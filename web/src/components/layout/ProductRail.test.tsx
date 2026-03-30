// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import ProductRail from './ProductRail';
import type { PlannerTask, ProductInfo } from '../../lib/types';

function makeTask(overrides: Partial<PlannerTask> = {}): PlannerTask {
  return {
    id: 1,
    title: 'Test Task',
    description: '',
    stage: 'backlog',
    priority: 1,
    product: 'chat',
    created_at: '2026-03-30T00:00:00Z',
    updated_at: '2026-03-30T00:00:00Z',
    ...overrides,
  } as PlannerTask;
}

const defaultProps = () => ({
  products: ['chat', 'tasks', 'scout'],
  activeProduct: null as string | null,
  tasks: [] as PlannerTask[],
  productMetadata: new Map<string, ProductInfo>(),
  onProductSelect: vi.fn(),
  expanded: false,
  onToggleExpanded: vi.fn(),
  settingsOpen: false,
  onSettingsToggle: vi.fn(),
  chatPosition: 'right' as const,
  setChatPosition: vi.fn(),
  tasksPosition: 'right' as const,
  setTasksPosition: vi.fn(),
  drawerLayout: 'independent' as const,
  setDrawerLayout: vi.fn(),
  autoInjectContext: true,
  setAutoInjectContext: vi.fn(),
  showContextChip: true,
  setShowContextChip: vi.fn(),
  toastsEnabled: true,
  setToastsEnabled: vi.fn(),
  inlineBadgesEnabled: true,
  setInlineBadgesEnabled: vi.fn(),
});

// jsdom doesn't implement scrollIntoView
Element.prototype.scrollIntoView = vi.fn();

describe('ProductRail', () => {
  afterEach(() => cleanup());

  it('renders navigation with proper aria', () => {
    render(<ProductRail {...defaultProps()} />);
    const rail = screen.getByTestId('product-rail');
    expect(rail.getAttribute('role')).toBe('navigation');
    expect(rail.getAttribute('aria-label')).toBe('Product navigation');
  });

  it('renders all products in collapsed mode', () => {
    render(<ProductRail {...defaultProps()} />);
    expect(screen.getByTestId('product-chat')).toBeTruthy();
    expect(screen.getByTestId('product-tasks')).toBeTruthy();
    expect(screen.getByTestId('product-scout')).toBeTruthy();
  });

  it('renders product abbreviations correctly', () => {
    render(<ProductRail {...defaultProps()} />);
    // "chat" => "CH", "tasks" => "TA", "scout" => "SC"
    expect(screen.getByTestId('product-chat').textContent).toContain('CH');
    expect(screen.getByTestId('product-tasks').textContent).toContain('TA');
    expect(screen.getByTestId('product-scout').textContent).toContain('SC');
  });

  it('uses product metadata labels for abbreviations', () => {
    const props = defaultProps();
    props.productMetadata = new Map([
      ['chat', { name: 'chat', label: 'Soul Chat' }],
    ]);
    render(<ProductRail {...props} />);
    // "Soul Chat" => "SC"
    expect(screen.getByTestId('product-chat').textContent).toContain('SC');
  });

  it('highlights active product', () => {
    render(<ProductRail {...defaultProps()} activeProduct="scout" />);
    const scoutBtn = screen.getByTestId('product-scout');
    expect(scoutBtn.className).toContain('text-soul');
    expect(scoutBtn.className).toContain('bg-soul/10');
  });

  it('calls onProductSelect when product clicked', () => {
    const props = defaultProps();
    render(<ProductRail {...props} />);
    fireEvent.click(screen.getByTestId('product-chat'));
    expect(props.onProductSelect).toHaveBeenCalledWith('chat');
  });

  it('shows active task count badges', () => {
    const props = defaultProps();
    props.tasks = [
      makeTask({ id: 1, product: 'chat', stage: 'active' }),
      makeTask({ id: 2, product: 'chat', stage: 'active' }),
      makeTask({ id: 3, product: 'chat', stage: 'blocked' }),
      makeTask({ id: 4, product: 'chat', stage: 'backlog' }),
    ];
    render(<ProductRail {...props} />);
    // 3 active/blocked tasks for chat
    const chatBtn = screen.getByTestId('product-chat');
    expect(chatBtn.textContent).toContain('3');
  });

  it('shows 9+ for counts above 9', () => {
    const props = defaultProps();
    props.tasks = Array.from({ length: 11 }, (_, i) =>
      makeTask({ id: i, product: 'chat', stage: 'active' })
    );
    render(<ProductRail {...props} />);
    const chatBtn = screen.getByTestId('product-chat');
    expect(chatBtn.textContent).toContain('9+');
  });

  it('does not show badge for zero active tasks', () => {
    const props = defaultProps();
    props.tasks = [makeTask({ id: 1, product: 'chat', stage: 'done' })];
    render(<ProductRail {...props} />);
    const chatBtn = screen.getByTestId('product-chat');
    // Only abbreviation text, no count
    expect(chatBtn.textContent?.trim()).toBe('CH');
  });

  it('renders expanded mode with labels', () => {
    const props = defaultProps();
    render(<ProductRail {...props} expanded={true} />);
    // In expanded mode, product labels are shown
    expect(screen.getByText('chat')).toBeTruthy();
    expect(screen.getByText('tasks')).toBeTruthy();
    expect(screen.getByText('scout')).toBeTruthy();
  });

  it('toggle expand/collapse button works', () => {
    const props = defaultProps();
    render(<ProductRail {...props} />);
    fireEvent.click(screen.getByTestId('product-rail-toggle'));
    expect(props.onToggleExpanded).toHaveBeenCalled();
  });

  it('toggle button has correct aria-expanded', () => {
    const props = defaultProps();
    render(<ProductRail {...props} expanded={true} />);
    const toggle = screen.getByTestId('product-rail-toggle');
    expect(toggle.getAttribute('aria-expanded')).toBe('true');
  });

  it('settings button exists and triggers toggle', () => {
    const props = defaultProps();
    render(<ProductRail {...props} />);
    fireEvent.click(screen.getByTestId('rail-settings-btn'));
    // When collapsed, should expand first then toggle settings
    expect(props.onToggleExpanded).toHaveBeenCalled();
    expect(props.onSettingsToggle).toHaveBeenCalled();
  });

  it('renders settings panel when settingsOpen and expanded', () => {
    const props = defaultProps();
    render(<ProductRail {...props} settingsOpen={true} expanded={true} />);
    // Settings content should show back button
    expect(screen.getByTestId('product-rail-settings-back')).toBeTruthy();
    expect(screen.getByText('Settings')).toBeTruthy();
  });

  it('settings back button calls onSettingsToggle', () => {
    const props = defaultProps();
    render(<ProductRail {...props} settingsOpen={true} expanded={true} />);
    fireEvent.click(screen.getByTestId('product-rail-settings-back'));
    expect(props.onSettingsToggle).toHaveBeenCalled();
  });

  it('renders Soul logo', () => {
    render(<ProductRail {...defaultProps()} />);
    const rail = screen.getByTestId('product-rail');
    // Diamond character used for Soul logo
    expect(rail.textContent).toContain('\u25C6');
  });

  it('renders with multi-word product names', () => {
    const props = defaultProps();
    props.products = ['compliance-go'];
    render(<ProductRail {...props} />);
    // "compliance-go" => "CG"
    expect(screen.getByTestId('product-compliance-go').textContent).toContain('CG');
  });

  it('non-active product lacks soul styling', () => {
    render(<ProductRail {...defaultProps()} activeProduct="scout" />);
    const chatBtn = screen.getByTestId('product-chat');
    expect(chatBtn.className).not.toContain('text-soul');
  });

  it('deselects active product when clicked again', () => {
    const props = defaultProps();
    render(<ProductRail {...props} activeProduct="chat" />);
    fireEvent.click(screen.getByTestId('product-chat'));
    expect(props.onProductSelect).toHaveBeenCalledWith('chat');
  });

  it('counts blocked tasks in badge', () => {
    const props = defaultProps();
    props.tasks = [
      makeTask({ id: 1, product: 'scout', stage: 'blocked' }),
      makeTask({ id: 2, product: 'scout', stage: 'active' }),
    ];
    render(<ProductRail {...props} />);
    const scoutBtn = screen.getByTestId('product-scout');
    expect(scoutBtn.textContent).toContain('2');
  });

  it('does not count backlog or brainstorm in badge', () => {
    const props = defaultProps();
    props.tasks = [
      makeTask({ id: 1, product: 'chat', stage: 'backlog' }),
      makeTask({ id: 2, product: 'chat', stage: 'brainstorm' }),
    ];
    render(<ProductRail {...props} />);
    const chatBtn = screen.getByTestId('product-chat');
    expect(chatBtn.textContent?.trim()).toBe('CH');
  });

  it('does not count done tasks in badge', () => {
    const props = defaultProps();
    props.tasks = [
      makeTask({ id: 1, product: 'tasks', stage: 'done' }),
    ];
    render(<ProductRail {...props} />);
    const tasksBtn = screen.getByTestId('product-tasks');
    expect(tasksBtn.textContent?.trim()).toBe('TA');
  });

  it('expanded mode shows product labels in lowercase', () => {
    const props = defaultProps();
    render(<ProductRail {...props} expanded={true} />);
    expect(screen.getByText('chat')).toBeTruthy();
  });

  it('shows correct abbreviation for single-word products', () => {
    const props = defaultProps();
    props.products = ['bench'];
    render(<ProductRail {...props} />);
    expect(screen.getByTestId('product-bench').textContent).toContain('BE');
  });

  it('renders settings content with AI Authentication heading', () => {
    const props = defaultProps();
    render(<ProductRail {...props} settingsOpen={true} expanded={true} />);
    expect(screen.getByText('AI Authentication')).toBeTruthy();
  });

  it('active product button has soul background color', () => {
    render(<ProductRail {...defaultProps()} activeProduct="chat" />);
    const chatBtn = screen.getByTestId('product-chat');
    expect(chatBtn.className).toContain('bg-soul');
  });

  it('inactive product button lacks soul background', () => {
    render(<ProductRail {...defaultProps()} activeProduct="scout" />);
    const chatBtn = screen.getByTestId('product-chat');
    expect(chatBtn.className).not.toContain('bg-soul/10');
  });
});
