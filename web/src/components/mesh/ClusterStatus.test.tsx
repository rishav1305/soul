// @vitest-environment jsdom
import { describe, it, expect, afterEach } from 'vitest';
import { render, screen, cleanup } from '@testing-library/react';
import { ClusterStatus } from './ClusterStatus';

function makeStatus(overrides: Record<string, unknown> = {}) {
  return {
    hub_id: 'hub-abc-123',
    total_nodes: 3,
    total_cpu: 12,
    total_ram_mb: 16384,
    total_storage_gb: 512,
    ...overrides,
  };
}

describe('ClusterStatus', () => {
  afterEach(() => cleanup());

  it('renders cluster status container', () => {
    render(<ClusterStatus status={makeStatus()} />);
    expect(screen.getByTestId('cluster-status')).toBeTruthy();
  });

  it('shows Cluster title', () => {
    render(<ClusterStatus status={makeStatus()} />);
    expect(screen.getByText('Cluster')).toBeTruthy();
  });

  it('shows hub id', () => {
    render(<ClusterStatus status={makeStatus({ hub_id: 'hub-xyz-789' })} />);
    expect(screen.getByTestId('cluster-hub-id').textContent).toContain('hub-xyz-789');
  });

  it('shows Healthy when all nodes have resources', () => {
    render(<ClusterStatus status={makeStatus({ total_nodes: 2, total_cpu: 8 })} />);
    expect(screen.getByTestId('cluster-health').textContent).toBe('Healthy');
  });

  it('shows No Nodes when total_nodes is 0', () => {
    render(<ClusterStatus status={makeStatus({ total_nodes: 0, total_cpu: 0, total_ram_mb: 0, total_storage_gb: 0 })} />);
    expect(screen.getByTestId('cluster-health').textContent).toBe('No Nodes');
  });

  it('shows total nodes stat', () => {
    render(<ClusterStatus status={makeStatus({ total_nodes: 5 })} />);
    expect(screen.getByTestId('mesh-stat-total-nodes')).toBeTruthy();
    expect(screen.getByText('5')).toBeTruthy();
  });

  it('shows total CPU stat', () => {
    render(<ClusterStatus status={makeStatus({ total_cpu: 24 })} />);
    expect(screen.getByTestId('mesh-stat-total-cpu')).toBeTruthy();
    expect(screen.getByText('24')).toBeTruthy();
  });

  it('shows total RAM in GB', () => {
    render(<ClusterStatus status={makeStatus({ total_ram_mb: 8192 })} />);
    expect(screen.getByText('8.0 GB')).toBeTruthy();
  });

  it('shows total storage', () => {
    render(<ClusterStatus status={makeStatus({ total_storage_gb: 1024 })} />);
    expect(screen.getByText('1024 GB')).toBeTruthy();
  });

  it('shows RAM in MB as sub-text', () => {
    render(<ClusterStatus status={makeStatus({ total_ram_mb: 16384 })} />);
    expect(screen.getByText('16,384 MB')).toBeTruthy();
  });
});
