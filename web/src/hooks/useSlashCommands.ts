import { useState, useEffect } from 'react';
import { authFetch } from '../lib/api.ts';

export interface SlashCommand {
  name: string;
  description: string;
  chatType?: string;
  builtin?: boolean;
}

const BUILTIN_COMMANDS: SlashCommand[] = [
  { name: 'think', description: 'Toggle extended thinking', builtin: true },
  { name: 'brainstorm', description: 'Brainstorm mode — clarify before implementing', chatType: 'Brainstorm' },
  { name: 'code', description: 'Code generation mode', chatType: 'Code' },
  { name: 'debug', description: 'Debug mode — systematic root cause analysis', chatType: 'Debug' },
  { name: 'architect', description: 'Architecture mode — design before building', chatType: 'Architect' },
  { name: 'review', description: 'Code review mode', chatType: 'Review' },
  { name: 'tdd', description: 'Test-driven development mode', chatType: 'TDD' },
];

export function useSlashCommands() {
  const [commands, setCommands] = useState<SlashCommand[]>(BUILTIN_COMMANDS);

  useEffect(() => {
    authFetch('/api/skills')
      .then(r => r.json())
      .then((skills: { name: string }[]) => {
        const builtinNames = new Set(BUILTIN_COMMANDS.map(b => b.name.toLowerCase()));
        const skillCommands: SlashCommand[] = skills
          .filter(s => !builtinNames.has(s.name.toLowerCase()))
          .map(s => ({
            name: s.name,
            description: `${s.name} skill`,
            chatType: s.name.charAt(0).toUpperCase() + s.name.slice(1),
          }));
        setCommands([...BUILTIN_COMMANDS, ...skillCommands]);
      })
      .catch(() => {}); // keep builtins if fetch fails
  }, []);

  return commands;
}
