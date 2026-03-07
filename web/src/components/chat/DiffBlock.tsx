interface DiffBlockProps {
  content: string;
}

export default function DiffBlock({ content }: DiffBlockProps) {
  const lines = content.split('\n');
  return (
    <div className="max-h-60 overflow-y-auto">
      <pre className="p-2 text-[11px] font-mono leading-relaxed">
        {lines.map((line, i) => {
          let cls = 'text-fg-muted';
          if (line.startsWith('+') && !line.startsWith('+++')) cls = 'text-green-400 bg-green-400/10';
          else if (line.startsWith('-') && !line.startsWith('---')) cls = 'text-red-400 bg-red-400/10';
          else if (line.startsWith('@@')) cls = 'text-soul/70';
          return (
            <div key={i} className={`px-2 ${cls}`}>
              {line || '\u00A0'}
            </div>
          );
        })}
      </pre>
    </div>
  );
}
