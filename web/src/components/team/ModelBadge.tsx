/** ModelBadge — pill badge indicating the Claude model variant an agent is using. */

export type ModelVariant = 'opus' | 'sonnet' | 'haiku' | string;

interface ModelBadgeProps {
  model: ModelVariant;
}

const STYLE: Record<string, string> = {
  opus:   'bg-cyan-900/40 text-cyan-400 border-cyan-800/40',
  sonnet: 'bg-green-900/40 text-green-400 border-green-800/40',
  haiku:  'bg-zinc-800/60 text-zinc-400 border-zinc-700/40',
};

const DEFAULT_STYLE = 'bg-zinc-800/60 text-zinc-400 border-zinc-700/40';

export function ModelBadge({ model }: ModelBadgeProps) {
  const style = STYLE[model.toLowerCase()] ?? DEFAULT_STYLE;
  return (
    <span
      data-testid={`model-badge-${model}`}
      className={`inline-block px-1.5 py-0.5 text-[10px] font-medium rounded border ${style} uppercase tracking-wide`}
    >
      {model}
    </span>
  );
}
