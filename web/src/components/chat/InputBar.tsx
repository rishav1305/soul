import { useState, useRef, useEffect, useCallback } from 'react';

interface InputBarProps {
  onSend: (message: string) => void;
  disabled: boolean;
}

export default function InputBar({ onSend, disabled }: InputBarProps) {
  const [value, setValue] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  useEffect(() => {
    textareaRef.current?.focus();
  }, []);

  const handleSend = useCallback(() => {
    const trimmed = value.trim();
    if (!trimmed || disabled) return;
    onSend(trimmed);
    setValue('');
    // Reset textarea height
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto';
    }
  }, [value, disabled, onSend]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend],
  );

  const handleChange = useCallback(
    (e: React.ChangeEvent<HTMLTextAreaElement>) => {
      setValue(e.target.value);
      // Auto-resize textarea
      const el = e.target;
      el.style.height = 'auto';
      el.style.height = `${Math.min(el.scrollHeight, 200)}px`;
    },
    [],
  );

  return (
    <div className="border-t border-zinc-800 p-4">
      <div className="flex items-end gap-2 max-w-3xl mx-auto">
        <textarea
          ref={textareaRef}
          value={value}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          disabled={disabled}
          placeholder="Message Soul..."
          rows={1}
          className="flex-1 bg-zinc-900 border border-zinc-700 rounded-lg px-4 py-3 text-zinc-100 placeholder-zinc-500 resize-none focus:outline-none focus:border-zinc-500 disabled:opacity-50 disabled:cursor-not-allowed"
        />
        <button
          onClick={handleSend}
          disabled={disabled || !value.trim()}
          className="px-4 py-3 bg-zinc-100 text-zinc-950 rounded-lg font-medium hover:bg-zinc-200 disabled:opacity-30 disabled:cursor-not-allowed transition-colors shrink-0"
        >
          Send
        </button>
      </div>
    </div>
  );
}
