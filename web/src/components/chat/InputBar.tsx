import { useState, useRef, useEffect, useCallback } from 'react';
import type { SendOptions } from '../../lib/types.ts';

interface ToolInfo {
  name: string;
  qualified_name: string;
  product: string;
  description: string;
  requires_approval: boolean;
}

interface ModelInfo {
  id: string;
  name: string;
  description: string;
}

interface InputBarProps {
  onSend: (message: string, options?: SendOptions) => void;
  disabled: boolean;
}

const CHAT_TYPES = ['Chat', 'Code', 'Planner'] as const;

export default function InputBar({ onSend, disabled }: InputBarProps) {
  const [value, setValue] = useState('');
  const [model, setModel] = useState<string>('');
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [chatType, setChatType] = useState<string>('Chat');
  const [tools, setTools] = useState<ToolInfo[]>([]);
  const [disabledTools, setDisabledTools] = useState<Set<string>>(new Set());
  const [showToolPopover, setShowToolPopover] = useState(false);
  const [files, setFiles] = useState<File[]>([]);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const imageInputRef = useRef<HTMLInputElement>(null);
  const toolPopoverRef = useRef<HTMLDivElement>(null);

  // Fetch models on mount
  useEffect(() => {
    fetch('/api/models')
      .then((r) => r.json())
      .then((data: ModelInfo[]) => {
        setModels(data);
        if (data.length > 0) setModel(data[0].id);
      })
      .catch(() => {});
  }, []);

  // Fetch tools on mount
  useEffect(() => {
    fetch('/api/tools')
      .then((r) => r.json())
      .then((data: ToolInfo[]) => {
        if (Array.isArray(data)) setTools(data);
      })
      .catch(() => {});
  }, []);

  // Auto-focus
  useEffect(() => {
    textareaRef.current?.focus();
  }, []);

  // Close tool popover on outside click
  useEffect(() => {
    if (!showToolPopover) return;
    const handler = (e: MouseEvent) => {
      if (toolPopoverRef.current && !toolPopoverRef.current.contains(e.target as Node)) {
        setShowToolPopover(false);
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, [showToolPopover]);

  const handleSend = useCallback(() => {
    const trimmed = value.trim();
    if (!trimmed || disabled) return;
    const options: SendOptions = {};
    if (model) options.model = model;
    if (chatType !== 'Chat') options.chatType = chatType.toLowerCase();
    if (disabledTools.size > 0) options.disabledTools = Array.from(disabledTools);
    onSend(trimmed, Object.keys(options).length > 0 ? options : undefined);
    setValue('');
    setFiles([]);
    if (textareaRef.current) textareaRef.current.style.height = 'auto';
  }, [value, disabled, model, chatType, disabledTools, onSend]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend],
  );

  const handleChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    setValue(e.target.value);
    const el = e.target;
    el.style.height = 'auto';
    el.style.height = `${Math.min(el.scrollHeight, 200)}px`;
  }, []);

  const toggleTool = useCallback((qualifiedName: string) => {
    setDisabledTools((prev) => {
      const next = new Set(prev);
      if (next.has(qualifiedName)) {
        next.delete(qualifiedName);
      } else {
        next.add(qualifiedName);
      }
      return next;
    });
  }, []);

  const handleFileSelect = useCallback((e: React.ChangeEvent<HTMLInputElement>) => {
    const selected = e.target.files;
    if (selected) setFiles((prev) => [...prev, ...Array.from(selected)]);
    e.target.value = '';
  }, []);

  const removeFile = useCallback((index: number) => {
    setFiles((prev) => prev.filter((_, i) => i !== index));
  }, []);

  return (
    <div className="px-5 py-4">
      <div className="glass rounded-2xl overflow-hidden">
        {/* File attachments */}
        {files.length > 0 && (
          <div className="flex flex-wrap gap-2 px-4 pt-3">
            {files.map((f, i) => (
              <span
                key={i}
                className="inline-flex items-center gap-1.5 bg-elevated rounded-lg px-2.5 py-1 text-xs text-fg-secondary"
              >
                {f.type.startsWith('image/') ? (
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5"><rect x="2" y="2" width="12" height="12" rx="2" /><circle cx="6" cy="6.5" r="1.5" /><path d="M2 11l3-3 2 2 3-3 4 4" /></svg>
                ) : (
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5"><path d="M9 2H4a1 1 0 0 0-1 1v10a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V6L9 2z" /><path d="M9 2v4h4" /></svg>
                )}
                <span className="max-w-[120px] truncate">{f.name}</span>
                <button
                  type="button"
                  onClick={() => removeFile(i)}
                  className="text-fg-muted hover:text-fg ml-0.5 cursor-pointer"
                >
                  ×
                </button>
              </span>
            ))}
          </div>
        )}

        {/* Textarea */}
        <textarea
          ref={textareaRef}
          value={value}
          onChange={handleChange}
          onKeyDown={handleKeyDown}
          disabled={disabled}
          placeholder="Message Soul..."
          rows={1}
          className="w-full bg-transparent px-4 pt-3 pb-2 text-fg placeholder:text-fg-muted font-body resize-none overflow-y-hidden focus:outline-none disabled:opacity-50 disabled:cursor-not-allowed"
        />

        {/* Toolbar */}
        <div className="flex items-center gap-1.5 px-3 py-2 border-t border-border-subtle">
          {/* Model selector */}
          {models.length > 0 && (
            <div className="relative">
              <select
                value={model}
                onChange={(e) => setModel(e.target.value)}
                className="soul-select pr-6 text-[11px]"
              >
                {models.map((m) => (
                  <option key={m.id} value={m.id}>
                    ◆ {m.name}
                  </option>
                ))}
              </select>
            </div>
          )}

          {/* Chat type */}
          <select
            value={chatType}
            onChange={(e) => setChatType(e.target.value)}
            className="soul-select pr-6 text-[11px]"
          >
            {CHAT_TYPES.map((t) => (
              <option key={t} value={t}>{t}</option>
            ))}
          </select>

          {/* Tool permissions */}
          <div className="relative" ref={toolPopoverRef}>
            <button
              type="button"
              onClick={() => setShowToolPopover(!showToolPopover)}
              className="relative w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
              title="Tool permissions"
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <path d="M6 3h4l2 2v2l-5 5-5-5V5l2-2z" />
                <circle cx="8" cy="6" r="1" />
              </svg>
              {disabledTools.size > 0 && (
                <span className="absolute -top-1 -right-1 w-4 h-4 bg-stage-blocked text-deep text-[9px] font-bold rounded-full flex items-center justify-center">
                  {disabledTools.size}
                </span>
              )}
            </button>

            {showToolPopover && tools.length > 0 && (
              <div className="absolute bottom-full left-0 mb-2 w-64 bg-surface border border-border-default rounded-xl shadow-xl z-40 py-2 max-h-60 overflow-y-auto">
                <div className="px-3 py-1.5 text-[10px] font-display uppercase tracking-widest text-fg-muted border-b border-border-subtle mb-1">
                  Tool Permissions
                </div>
                {tools.map((tool) => {
                  const isDisabled = disabledTools.has(tool.qualified_name);
                  return (
                    <button
                      key={tool.qualified_name}
                      type="button"
                      onClick={() => toggleTool(tool.qualified_name)}
                      className="w-full flex items-center gap-2 px-3 py-1.5 hover:bg-elevated transition-colors text-left cursor-pointer"
                    >
                      <span className={`w-3.5 h-3.5 rounded border flex items-center justify-center shrink-0 transition-colors ${
                        isDisabled
                          ? 'border-fg-muted bg-transparent'
                          : 'border-soul bg-soul'
                      }`}>
                        {!isDisabled && (
                          <svg width="8" height="8" viewBox="0 0 10 10" fill="none" stroke="var(--color-deep)" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                            <path d="M2 5l2.5 2.5L8 3" />
                          </svg>
                        )}
                      </span>
                      <span className="flex-1 min-w-0">
                        <span className={`text-xs block truncate ${isDisabled ? 'text-fg-muted' : 'text-fg'}`}>
                          {tool.name}
                        </span>
                        <span className="text-[10px] text-fg-muted block truncate">{tool.product}</span>
                      </span>
                      {tool.requires_approval && (
                        <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="var(--color-stage-validation)" strokeWidth="1.5" className="shrink-0">
                          <path d="M8 1.5l6 3v4c0 3.5-2.5 5.5-6 7-3.5-1.5-6-3.5-6-7v-4l6-3z" />
                        </svg>
                      )}
                    </button>
                  );
                })}
              </div>
            )}
          </div>

          {/* File attach */}
          <button
            type="button"
            onClick={() => fileInputRef.current?.click()}
            className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
            title="Attach file"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M13.5 7.5l-5.8 5.8a3.5 3.5 0 0 1-5-5l5.8-5.8a2.3 2.3 0 0 1 3.3 3.3L6 11.6a1.2 1.2 0 0 1-1.7-1.7L9.5 4.7" />
            </svg>
          </button>
          <input ref={fileInputRef} type="file" multiple className="hidden" onChange={handleFileSelect} />

          {/* Image attach */}
          <button
            type="button"
            onClick={() => imageInputRef.current?.click()}
            className="w-7 h-7 flex items-center justify-center rounded hover:bg-elevated text-fg-muted hover:text-fg transition-colors cursor-pointer"
            title="Attach image"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <rect x="2" y="2" width="12" height="12" rx="2" />
              <circle cx="5.5" cy="5.5" r="1.5" />
              <path d="M14 10l-3-3-7 7" />
            </svg>
          </button>
          <input ref={imageInputRef} type="file" accept="image/*" multiple className="hidden" onChange={handleFileSelect} />

          {/* Spacer */}
          <div className="flex-1" />

          {/* Send button */}
          <button
            onClick={handleSend}
            disabled={disabled || !value.trim()}
            className="w-8 h-8 bg-soul text-deep rounded-full flex items-center justify-center hover:bg-soul/85 disabled:opacity-20 disabled:cursor-not-allowed transition-colors shrink-0 cursor-pointer"
            title="Send"
          >
            <svg width="16" height="16" viewBox="0 0 16 16" fill="currentColor">
              <path d="M8 3l-1 1 3.3 3.3H3v1.4h7.3L7 12l1 1 5-5-5-5z" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  );
}
