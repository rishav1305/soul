// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { NodeList } from './NodeList';

function makeNode(overrides: Record<string, unknown> = {}) {
  return {
    id: 'node-1',
    name: 'titan-pi',
    role: 'hub',
    status: 'online',
    platform: 'linux',
    arch: 'arm64',
    last_heartbeat: new Date().toISOString(),
    capability_score: 45,
    ...overrides,
  };
}

describe('NodeList', () => {
  afterEach(() => cleanup());

  it('shows empty state when no nodes', () => {
    render(<NodeList nodes={[]} selectedId={null} onSelect={vi.fn()} />);
    expect(screen.getByTestId('node-list-empty')).toBeTruthy();
    expect(screen.getByText('No nodes found.')).toBeTruthy();
  });

  it('renders node list table', () => {
    render(<NodeList nodes={[makeNode()]} selectedId={null} onSelect={vi.fn()} />);
    expect(screen.getByTestId('node-list')).toBeTruthy();
  });

  it('renders node row with testid', () => {
    render(<NodeList nodes={[makeNode({ id: 'node-42' })]} selectedId={null} onSelect={vi.fn()} />);
    expect(screen.getByTestId('node-row-node-42')).toBeTruthy();
  });

  it('shows node name', () => {
    render(<NodeList nodes={[makeNode({ name: 'titan-pc' })]} selectedId={null} onSelect={vi.fn()} />);
    expect(screen.getByText('titan-pc')).toBeTruthy();
  });

  it('shows node role', () => {
    render(<NodeList nodes={[makeNode({ role: 'agent' })]} selectedId={null} onSelect={vi.fn()} />);
    expect(screen.getByText('agent')).toBeTruthy();
  });

  it('shows node status', () => {
    render(<NodeList nodes={[makeNode({ status: 'online' })]} selectedId={null} onSelect={vi.fn()} />);
    expect(screen.getByText('online')).toBeTruthy();
  });

  it('shows capability score', () => {
    render(<NodeList nodes={[makeNode({ capability_score: 60 })]} selectedId={null} onSelect={vi.fn()} />);
    expect(screen.getByText('60')).toBeTruthy();
  });

  it('shows -- for null capability score', () => {
    render(<NodeList nodes={[makeNode({ capability_score: null })]} selectedId={null} onSelect={vi.fn()} />);
    expect(screen.getByText('--')).toBeTruthy();
  });

  it('calls onSelect when node row clicked', () => {
    const onSelect = vi.fn();
    const node = makeNode({ id: 'node-5' });
    render(<NodeList nodes={[node]} selectedId={null} onSelect={onSelect} />);
    fireEvent.click(screen.getByTestId('node-row-node-5'));
    expect(onSelect).toHaveBeenCalledWith(node);
  });

  it('renders column headers', () => {
    render(<NodeList nodes={[makeNode()]} selectedId={null} onSelect={vi.fn()} />);
    expect(screen.getByText('Name')).toBeTruthy();
    expect(screen.getByText('Role')).toBeTruthy();
    expect(screen.getByText('Status')).toBeTruthy();
    expect(screen.getByText('Score')).toBeTruthy();
  });

  it('shows platform/arch info', () => {
    render(<NodeList nodes={[makeNode({ platform: 'linux', arch: 'arm64' })]} selectedId={null} onSelect={vi.fn()} />);
    expect(screen.getByText('linux/arm64')).toBeTruthy();
  });
});
