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

  it('adds guardrail on Enter key press', () => {
    render(<SandboxConfig config={{ ...defaultConfig(), guardrails: [] }} onSave={vi.fn()} />);
    const input = screen.getByTestId('guardrail-input');
    fireEvent.change(input, { target: { value: 'New rule via Enter' } });
    fireEvent.keyDown(input, { key: 'Enter' });
    expect(screen.getByText('New rule via Enter')).toBeTruthy();
  });

  it('does not add duplicate guardrail', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    // "No sensitive data" already exists
    fireEvent.change(screen.getByTestId('guardrail-input'), { target: { value: 'No sensitive data' } });
    fireEvent.click(screen.getByTestId('guardrail-add'));
    // Should still have only 1 instance
    const matches = screen.getAllByText('No sensitive data');
    expect(matches).toHaveLength(1);
  });

  it('does not add empty/whitespace-only guardrail', () => {
    render(<SandboxConfig config={{ ...defaultConfig(), guardrails: [] }} onSave={vi.fn()} />);
    fireEvent.change(screen.getByTestId('guardrail-input'), { target: { value: '   ' } });
    fireEvent.click(screen.getByTestId('guardrail-add'));
    // No guardrail tags rendered (the flex-wrap container won't appear)
    expect(screen.queryByTestId('guardrail-remove-0')).toBeNull();
  });

  it('clears guardrail input after adding', () => {
    render(<SandboxConfig config={{ ...defaultConfig(), guardrails: [] }} onSave={vi.fn()} />);
    const input = screen.getByTestId('guardrail-input') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'Rule 1' } });
    fireEvent.click(screen.getByTestId('guardrail-add'));
    expect(input.value).toBe('');
  });

  it('shows multiple guardrails with remove buttons', () => {
    render(<SandboxConfig config={{ ...defaultConfig(), guardrails: ['Rule A', 'Rule B', 'Rule C'] }} onSave={vi.fn()} />);
    expect(screen.getByText('Rule A')).toBeTruthy();
    expect(screen.getByText('Rule B')).toBeTruthy();
    expect(screen.getByText('Rule C')).toBeTruthy();
    expect(screen.getByTestId('guardrail-remove-0')).toBeTruthy();
    expect(screen.getByTestId('guardrail-remove-1')).toBeTruthy();
    expect(screen.getByTestId('guardrail-remove-2')).toBeTruthy();
  });

  it('selected weakness level has active styling', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    const mediumBtn = screen.getByTestId('weakness-medium');
    expect(mediumBtn.className).toContain('border-soul');
  });

  it('non-selected weakness level lacks active styling', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    const noneBtn = screen.getByTestId('weakness-none');
    expect(noneBtn.className).not.toContain('bg-soul-dim');
  });

  it('shows Saving... during save operation', async () => {
    let resolveSave: () => void;
    const onSave = vi.fn().mockImplementation(() => new Promise<void>(r => { resolveSave = r; }));
    render(<SandboxConfig config={defaultConfig()} onSave={onSave} />);
    await act(async () => {
      fireEvent.click(screen.getByTestId('sandbox-save'));
    });
    expect(screen.getByText('Saving...')).toBeTruthy();
    await act(async () => { resolveSave!(); });
    expect(screen.getByText('Save Configuration')).toBeTruthy();
  });

  it('save passes all current form values', async () => {
    const onSave = vi.fn().mockResolvedValue(undefined);
    render(<SandboxConfig config={defaultConfig()} onSave={onSave} />);
    // Modify name
    fireEvent.change(screen.getByTestId('sandbox-name'), { target: { value: 'Updated Name' } });
    // Modify prompt
    fireEvent.change(screen.getByTestId('sandbox-prompt'), { target: { value: 'Be concise.' } });
    // Change weakness
    fireEvent.click(screen.getByTestId('weakness-high'));
    await act(async () => {
      fireEvent.click(screen.getByTestId('sandbox-save'));
    });
    expect(onSave).toHaveBeenCalledWith({
      name: 'Updated Name',
      systemPrompt: 'Be concise.',
      guardrails: ['No sensitive data'],
      weaknessLevel: 'high',
    });
  });

  it('updates weakness description when level changes', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    fireEvent.click(screen.getByTestId('weakness-none'));
    expect(screen.getByText('Fully hardened, maximum guardrails')).toBeTruthy();
    fireEvent.click(screen.getByTestId('weakness-low'));
    expect(screen.getByText('Slight vulnerability to social engineering')).toBeTruthy();
  });

  it('name input can be updated', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    const input = screen.getByTestId('sandbox-name') as HTMLInputElement;
    fireEvent.change(input, { target: { value: 'New Name' } });
    expect(input.value).toBe('New Name');
  });

  it('system prompt textarea can be updated', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    const textarea = screen.getByTestId('sandbox-prompt') as HTMLTextAreaElement;
    fireEvent.change(textarea, { target: { value: 'New prompt text' } });
    expect(textarea.value).toBe('New prompt text');
  });

  it('disables add button when input is only whitespace', () => {
    render(<SandboxConfig config={defaultConfig()} onSave={vi.fn()} />);
    fireEvent.change(screen.getByTestId('guardrail-input'), { target: { value: '   ' } });
    // The button checks newGuardrail.trim(), but the disabled prop checks !newGuardrail.trim()
    // With whitespace, trim() returns '', so disabled should be true
    expect((screen.getByTestId('guardrail-add') as HTMLButtonElement).disabled).toBe(true);
  });

  it('removes second guardrail correctly', () => {
    render(<SandboxConfig config={{ ...defaultConfig(), guardrails: ['First', 'Second', 'Third'] }} onSave={vi.fn()} />);
    fireEvent.click(screen.getByTestId('guardrail-remove-1'));
    expect(screen.queryByText('Second')).toBeNull();
    expect(screen.getByText('First')).toBeTruthy();
    expect(screen.getByText('Third')).toBeTruthy();
  });
});
