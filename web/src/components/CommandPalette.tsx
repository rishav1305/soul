import { useState, useEffect, useCallback } from 'react';

export interface SlashCommand {
  name: string;
  description: string;
}

interface CommandPaletteProps {
  commands: SlashCommand[];
  filter: string;
  onSelect: (cmd: SlashCommand) => void;
  onClose: () => void;
}

export function CommandPalette({ commands, filter, onSelect, onClose }: CommandPaletteProps) {
  const filtered = commands.filter(c =>
    c.name.toLowerCase().includes(filter.toLowerCase()),
  );
  const [selected, setSelected] = useState(0);

  useEffect(() => {
    setSelected(0);
  }, [filter]);

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (filtered.length === 0) return;
      if (e.key === 'ArrowDown') {
        e.preventDefault();
        setSelected(i => (i + 1) % filtered.length);
      } else if (e.key === 'ArrowUp') {
        e.preventDefault();
        setSelected(i => (i - 1 + filtered.length) % filtered.length);
      } else if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault();
        onSelect(filtered[selected]);
      } else if (e.key === 'Escape') {
        e.preventDefault();
        onClose();
      }
    },
    [filtered, selected, onSelect, onClose],
  );

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);

  if (filtered.length === 0) return null;

  return (
    <div
      data-testid="command-palette"
      className="absolute bottom-full left-0 right-0 mb-1 mx-3 bg-elevated border border-border-default rounded-lg shadow-xl overflow-hidden z-20"
    >
      {filtered.map((cmd, i) => (
        <button
          key={cmd.name}
          type="button"
          data-testid={`command-${cmd.name}`}
          onClick={() => onSelect(cmd)}
          className={`w-full text-left px-3 py-2 flex items-center gap-3 text-sm transition-colors ${
            i === selected ? 'bg-overlay text-fg' : 'text-fg-secondary hover:bg-overlay/50'
          }`}
        >
          <span className="font-mono text-soul">/{cmd.name}</span>
          <span className="text-fg-muted text-xs">{cmd.description}</span>
        </button>
      ))}
    </div>
  );
}
