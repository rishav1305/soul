// @vitest-environment happy-dom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup } from '@testing-library/react';
import { NodeDetail } from './NodeDetail';

function makeNode(overrides: Record<string, unknown> = {}) {
  return {
    id: 'node-1',
    name: 'titan-pi',
    role: 'hub',
    status: 'online',
    platform: 'linux',
    arch: 'arm64',
    host: '192.168.0.128',
    port: 3024,
    cpu_cores: 4,
    ram_total_mb: 8192,
    storage_total_gb: 64,
    capability_score: 45,
    last_heartbeat: new Date().toISOString(),
    ...overrides,
  };
}

function makeHeartbeat(overrides: Record<string, unknown> = {}) {
  return {
    timestamp: new Date().toISOString(),
    cpu_usage_percent: 35,
    ram_used_percent: 60,
    ram_available_mb: 3200,
    storage_free_gb: 40,
    ...overrides,
  };
}

describe('NodeDetail', () => {
  afterEach(() => cleanup());

  it('renders node detail container', () => {
    render(<NodeDetail node={makeNode()} heartbeats={[]} onClose={vi.fn()} />);
    expect(screen.getByTestId('node-detail-node-1')).toBeTruthy();
  });

  it('shows node name', () => {
    render(<NodeDetail node={makeNode({ name: 'titan-pc' })} heartbeats={[]} onClose={vi.fn()} />);
    expect(screen.getByText('titan-pc')).toBeTruthy();
  });

  it('shows role badge', () => {
    render(<NodeDetail node={makeNode({ role: 'agent' })} heartbeats={[]} onClose={vi.fn()} />);
    expect(screen.getByText('agent')).toBeTruthy();
  });

  it('shows status', () => {
    render(<NodeDetail node={makeNode({ status: 'online' })} heartbeats={[]} onClose={vi.fn()} />);
    expect(screen.getByText('online')).toBeTruthy();
  });

  it('renders close button', () => {
    render(<NodeDetail node={makeNode()} heartbeats={[]} onClose={vi.fn()} />);
    expect(screen.getByTestId('node-detail-close')).toBeTruthy();
  });

  it('calls onClose when close clicked', () => {
    const onClose = vi.fn();
    render(<NodeDetail node={makeNode()} heartbeats={[]} onClose={onClose} />);
    fireEvent.click(screen.getByTestId('node-detail-close'));
    expect(onClose).toHaveBeenCalled();
  });

  it('shows platform info', () => {
    render(<NodeDetail node={makeNode({ platform: 'linux' })} heartbeats={[]} onClose={vi.fn()} />);
    const platformCard = screen.getByTestId('node-platform');
    expect(platformCard.textContent).toContain('linux');
  });

  it('shows architecture info', () => {
    render(<NodeDetail node={makeNode({ arch: 'arm64' })} heartbeats={[]} onClose={vi.fn()} />);
    const archCard = screen.getByTestId('node-arch');
    expect(archCard.textContent).toContain('arm64');
  });

  it('shows host:port', () => {
    render(<NodeDetail node={makeNode({ host: '192.168.0.128', port: 3024 })} heartbeats={[]} onClose={vi.fn()} />);
    const hostCard = screen.getByTestId('node-host');
    expect(hostCard.textContent).toContain('192.168.0.128:3024');
  });

  it('shows capability score', () => {
    render(<NodeDetail node={makeNode({ capability_score: 60 })} heartbeats={[]} onClose={vi.fn()} />);
    const scoreCard = screen.getByTestId('node-score');
    expect(scoreCard.textContent).toContain('60');
  });

  it('shows CPU cores', () => {
    render(<NodeDetail node={makeNode({ cpu_cores: 8 })} heartbeats={[]} onClose={vi.fn()} />);
    const cpuCard = screen.getByTestId('node-cpu');
    expect(cpuCard.textContent).toContain('8');
  });

  it('shows RAM in GB', () => {
    render(<NodeDetail node={makeNode({ ram_total_mb: 16384 })} heartbeats={[]} onClose={vi.fn()} />);
    const ramCard = screen.getByTestId('node-ram');
    expect(ramCard.textContent).toContain('16.0 GB');
  });

  it('shows storage', () => {
    render(<NodeDetail node={makeNode({ storage_total_gb: 256 })} heartbeats={[]} onClose={vi.fn()} />);
    const storageCard = screen.getByTestId('node-storage');
    expect(storageCard.textContent).toContain('256 GB');
  });

  it('shows no heartbeat message when empty', () => {
    render(<NodeDetail node={makeNode()} heartbeats={[]} onClose={vi.fn()} />);
    expect(screen.getByTestId('no-heartbeats')).toBeTruthy();
  });

  it('shows heartbeat chart when heartbeats present', () => {
    render(<NodeDetail node={makeNode()} heartbeats={[makeHeartbeat()]} onClose={vi.fn()} />);
    expect(screen.getByTestId('heartbeat-chart')).toBeTruthy();
    expect(screen.getByTestId('heartbeat-0')).toBeTruthy();
  });
});
