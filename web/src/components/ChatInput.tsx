import { useState, useCallback, useRef, useEffect } from 'react';
import type { KeyboardEvent, ChangeEvent } from 'react';
import type { ChatInputProps } from '../lib/types';

export function ChatInput({ onSend, disabled }: ChatInputProps) {
  const [value, setValue] = useState('');
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Focus textarea on mount.
  useEffect(() => {
    textareaRef.current?.focus();
  }, []);

  // Auto-resize textarea to fit content.
  const resizeTextarea = useCallback(() => {
    const ta = textareaRef.current;
    if (!ta) return;
    ta.style.height = 'auto';
    ta.style.height = `${Math.min(ta.scrollHeight, 200)}px`;
  }, []);

  const handleChange = useCallback(
    (e: ChangeEvent<HTMLTextAreaElement>) => {
      setValue(e.target.value);
      resizeTextarea();
    },
    [resizeTextarea],
  );

  const handleSend = useCallback(() => {
    const trimmed = value.trim();
    if (!trimmed || disabled) return;
    onSend(trimmed);
    setValue('');
    // Reset textarea height after clearing.
    requestAnimationFrame(() => {
      const ta = textareaRef.current;
      if (ta) {
        ta.style.height = 'auto';
      }
    });
  }, [value, disabled, onSend]);

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend],
  );

  return (
    <div className="flex items-end gap-2 p-4 border-t border-zinc-800">
      <textarea
        ref={textareaRef}
        data-testid="chat-input"
        className="flex-1 resize-none rounded-lg bg-zinc-800 px-4 py-3 text-zinc-100 placeholder-zinc-500 outline-none focus:ring-2 focus:ring-zinc-600 disabled:opacity-50 disabled:cursor-not-allowed"
        placeholder="Send a message..."
        rows={1}
        value={value}
        onChange={handleChange}
        onKeyDown={handleKeyDown}
        disabled={disabled}
      />
      <button
        data-testid="send-button"
        className="rounded-lg bg-zinc-700 px-4 py-3 text-zinc-100 hover:bg-zinc-600 transition-colors disabled:opacity-50 disabled:cursor-not-allowed"
        onClick={handleSend}
        disabled={disabled || !value.trim()}
        type="button"
      >
        Send
      </button>
    </div>
  );
}
