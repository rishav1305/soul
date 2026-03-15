import { useState } from 'react';
import type { SandboxConfig as SandboxConfigType } from '../../hooks/useSentinel';

interface SandboxConfigProps {
  config: SandboxConfigType;
  onSave: (config: SandboxConfigType) => Promise<void>;
}

const weaknessLevels: SandboxConfigType['weaknessLevel'][] = ['none', 'low', 'medium', 'high'];

const weaknessDescriptions: Record<string, string> = {
  none: 'Fully hardened, maximum guardrails',
  low: 'Slight vulnerability to social engineering',
  medium: 'Moderate weaknesses in prompt handling',
  high: 'Minimal defenses, easily exploitable',
};

export function SandboxConfig({ config, onSave }: SandboxConfigProps) {
  const [name, setName] = useState(config.name);
  const [systemPrompt, setSystemPrompt] = useState(config.systemPrompt);
  const [guardrails, setGuardrails] = useState<string[]>(config.guardrails);
  const [weaknessLevel, setWeaknessLevel] = useState<SandboxConfigType['weaknessLevel']>(config.weaknessLevel);
  const [newGuardrail, setNewGuardrail] = useState('');
  const [saving, setSaving] = useState(false);

  const handleAddGuardrail = () => {
    const value = newGuardrail.trim();
    if (!value || guardrails.includes(value)) return;
    setGuardrails(prev => [...prev, value]);
    setNewGuardrail('');
  };

  const handleRemoveGuardrail = (index: number) => {
    setGuardrails(prev => prev.filter((_, i) => i !== index));
  };

  const handleGuardrailKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault();
      handleAddGuardrail();
    }
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await onSave({ name, systemPrompt, guardrails, weaknessLevel });
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="bg-surface rounded-lg p-4 space-y-4" data-testid="sandbox-config">
      {/* Name */}
      <div className="space-y-1">
        <label className="text-xs text-fg-muted font-medium">Sandbox Name</label>
        <input
          type="text"
          value={name}
          onChange={e => setName(e.target.value)}
          placeholder="My Sandbox"
          className="w-full bg-elevated border border-border-default rounded-lg px-3 py-2 text-sm text-fg placeholder:text-fg-muted focus:outline-none focus:border-soul"
          data-testid="sandbox-name"
        />
      </div>

      {/* System Prompt */}
      <div className="space-y-1">
        <label className="text-xs text-fg-muted font-medium">System Prompt</label>
        <textarea
          value={systemPrompt}
          onChange={e => setSystemPrompt(e.target.value)}
          placeholder="You are a helpful assistant that..."
          rows={4}
          className="w-full bg-elevated border border-border-default rounded-lg px-3 py-2 text-sm text-fg placeholder:text-fg-muted focus:outline-none focus:border-soul resize-y"
          data-testid="sandbox-prompt"
        />
      </div>

      {/* Guardrails */}
      <div className="space-y-1">
        <label className="text-xs text-fg-muted font-medium">Guardrails</label>
        <div className="flex gap-2">
          <input
            type="text"
            value={newGuardrail}
            onChange={e => setNewGuardrail(e.target.value)}
            onKeyDown={handleGuardrailKeyDown}
            placeholder="Add a guardrail rule..."
            className="flex-1 bg-elevated border border-border-default rounded-lg px-3 py-2 text-sm text-fg placeholder:text-fg-muted focus:outline-none focus:border-soul"
            data-testid="guardrail-input"
          />
          <button
            onClick={handleAddGuardrail}
            disabled={!newGuardrail.trim()}
            className="px-3 py-2 text-xs rounded-lg bg-elevated hover:bg-overlay text-fg-secondary disabled:opacity-40 transition-colors"
            data-testid="guardrail-add"
          >
            Add
          </button>
        </div>
        {guardrails.length > 0 && (
          <div className="flex flex-wrap gap-1.5 mt-2">
            {guardrails.map((rule, i) => (
              <span key={i} className="inline-flex items-center gap-1 px-2 py-0.5 text-xs rounded-full bg-blue-500/20 text-blue-400">
                {rule}
                <button
                  onClick={() => handleRemoveGuardrail(i)}
                  className="hover:text-red-400 transition-colors"
                  data-testid={`guardrail-remove-${i}`}
                >
                  &#215;
                </button>
              </span>
            ))}
          </div>
        )}
      </div>

      {/* Weakness Level */}
      <div className="space-y-2">
        <label className="text-xs text-fg-muted font-medium">Weakness Level</label>
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-2" data-testid="sandbox-weakness">
          {weaknessLevels.map(level => (
            <button
              key={level}
              onClick={() => setWeaknessLevel(level)}
              className={`px-3 py-2 text-xs rounded-lg border transition-colors capitalize ${
                weaknessLevel === level
                  ? 'border-soul bg-soul-dim text-soul'
                  : 'border-border-default bg-elevated text-fg-secondary hover:border-border-active'
              }`}
              data-testid={`weakness-${level}`}
            >
              {level}
            </button>
          ))}
        </div>
        <p className="text-[10px] text-fg-muted">{weaknessDescriptions[weaknessLevel]}</p>
      </div>

      {/* Save */}
      <button
        onClick={handleSave}
        disabled={saving || !name.trim()}
        className="px-4 py-2 text-sm rounded-lg bg-soul text-deep font-medium hover:bg-soul/85 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
        data-testid="sandbox-save"
      >
        {saving ? 'Saving...' : 'Save Configuration'}
      </button>
    </div>
  );
}
