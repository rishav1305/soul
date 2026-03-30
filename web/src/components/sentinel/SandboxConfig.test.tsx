// @vitest-environment jsdom
import { describe, it, expect, vi, afterEach } from 'vitest';
import { render, screen, fireEvent, cleanup, act } from '@testing-library/react';
import { SandboxConfig } from './SandboxConfig';

const defaultConfig = () => ({
  name: 'Test Sandbox',
  systemPrompt: 'You are a helpful assistant.',
  guardrails: ['No sensitive data'],
  weaknessLevel: 'medium' as const,
});

describe('SandboxConfig', () => {
  afterEach(() => cleanup());

  it('renders sandbox config container', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    expect(screen.getByTestId('sandbox-config')).toBeTruthy();
  });

  it('renders name input with initial value', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    expect((screen.getByTestId('sandbox-name') as HTMLInputElement).value).toBe('Test Sandbox');
  });

  it('renders system prompt textarea', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    expect((screen.getByTestId('sandbox-prompt') as HTMLTextAreaElement).value).toBe('You are a helpful assistant.');
  });

  it('renders guardrail input', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    expect(screen.getByTestId('guardrail-input')).toBeTruthy();
    expect(screen.getByTestId('guardrail-add')).toBeTruthy();
  });

  it('shows existing guardrails', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    expect(screen.getByText('No sensitive data')).toBeTruthy();
  });

  it('adds guardrail when Add clicked', () => {
    render(<SandboxConfig config={{ ...defaultConfig(), guardrails: [] }} onSave={vi.fn()} />);
    fireEvent.change(screen.getByTestId('guardrail-input'), { target: { value: 'New rule' } });
    fireEvent.click(screen.getByTestId('guardrail-add'));
    expect(screen.getByText('New rule')).toBeTruthy();
  });

  it('removes guardrail when remove clicked', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    fireEvent.click(screen.getByTestId('guardrail-remove-0'));
    expect(screen.queryByText('No sensitive data')).toBeNull();
  });

  it('renders weakness level buttons', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    expect(screen.getByTestId('weakness-none')).toBeTruthy();
    expect(screen.getByTestId('weakness-low')).toBeTruthy();
    expect(screen.getByTestId('weakness-medium')).toBeTruthy();
    expect(screen.getByTestId('weakness-high')).toBeTruthy();
  });

  it('shows description for selected weakness level', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    expect(screen.getByText('Moderate weaknesses in prompt handling')).toBeTruthy();
  });

  it('changes weakness level when button clicked', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    fireEvent.click(screen.getByTestId('weakness-high'));
    expect(screen.getByText('Minimal defenses, easily exploitable')).toBeTruthy();
  });

  it('renders save button', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    expect(screen.getByTestId('sandbox-save')).toBeTruthy();
    expect(screen.getByText('Save Configuration')).toBeTruthy();
  });

  it('calls onSave with config when Save clicked', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(<SandboxConfig config={defaultConfig()} onSave={onSave} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('sandbox-save'));
    });
    expect(onSave).toHaveBeenCalledWith(expect.objectContaining({
      name: 'Test Sandbox',
      weaknessLevel: 'medium',
    }));
  });

  it('disables save when name is empty', () => {
    render(<SandboxConfig config={{ ...defaultConfig(), name: '' }} onSave={vi.fn()} />);
    expect((screen.getByTestId('sandbox-save') as HTMLButtonElement).disabled).toBe(true);
  });

  it('add button disabled when guardrail input is empty', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    expect((screen.getByTestId('guardrail-add') as HTMLButtonElement).disabled).toBe(true);
  });
});
