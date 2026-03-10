interface DiffBlockProps {
  content: string;
}

export function DiffBlock({ content }: DiffBlockProps) {
  return (
    <pre className="text-[11px] font-mono max-h-60 overflow-y-auto p-2">
      {content.split('\n').map((line, i) => {
        let cls = 'text-fg-muted';
        if (line.startsWith('+')) cls = 'text-green-400 bg-green-400/10';
        else if (line.startsWith('-')) cls = 'text-red-400 bg-red-400/10';
        else if (line.startsWith('@@')) cls = 'text-soul/70';
        return (
          <div key={i} className={cls}>
            {line || '\u00a0'}
          </div>
        );
      })}
    </pre>
  );
}
