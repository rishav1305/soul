import { useState, useRef, useEffect, useCallback, useMemo } from 'react';
import type { SendOptions } from '../../lib/types.ts';
import { useSlashCommands } from '../../hooks/useSlashCommands.ts';
import type { SlashCommand } from '../../hooks/useSlashCommands.ts';

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

const CHAT_TYPES = [
  { value: 'Chat', label: 'Chat', group: 'mode' },
  { value: 'Code', label: 'Code', group: 'mode' },
  { value: 'Architect', label: 'Architect', group: 'mode' },
  { value: 'Debug', label: 'Debug', group: 'skill' },
  { value: 'Review', label: 'Review', group: 'skill' },
  { value: 'TDD', label: 'TDD', group: 'skill' },
  { value: 'Brainstorm', label: 'Brainstorm', group: 'skill' },
] as const;

export default function InputBar({ onSend, disabled }: InputBarProps) {
  const [value, setValue] = useState('');
  const [model, setModel] = useState<string>('');
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [chatType, setChatType] = useState<string>('Chat');
  const [thinking, setThinking] = useState(false);
  const [tools, setTools] = useState<ToolInfo[]>([]);
  const [disabledTools, setDisabledTools] = useState<Set<string>>(new Set());
  const [showToolPopover, setShowToolPopover] = useState(false);
  const [files, setFiles] = useState<File[]>([]);
  const [isListening, setIsListening] = useState(false);
  const [interimText, setInterimText] = useState('');
  const [slashQuery, setSlashQuery] = useState('');
  const [showSlashPalette, setShowSlashPalette] = useState(false);
  const [paletteIndex, setPaletteIndex] = useState(0);
  const commands = useSlashCommands();
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const imageInputRef = useRef<HTMLInputElement>(null);
  const toolPopoverRef = useRef<HTMLDivElement>(null);
  const recognitionRef = useRef<any>(null);
  const speechSupported = typeof window !== 'undefined' && ('webkitSpeechRecognition' in window || 'SpeechRecognition' in window);

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
    if (thinking) options.thinking = true;
    onSend(trimmed, Object.keys(options).length > 0 ? options : undefined);
    setValue('');
    setFiles([]);
    if (textareaRef.current) textareaRef.current.style.height = 'auto';
  }, [value, disabled, model, chatType, disabledTools, thinking, onSend]);

  const handleChange = useCallback((e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value;
    setValue(val);
    const el = e.target;
    el.style.height = 'auto';
    el.style.height = `${Math.min(el.scrollHeight, 200)}px`;
    // Slash command palette: show when input starts with / and has no space
    if (val.startsWith('/') && !val.includes(' ')) {
      setSlashQuery(val.slice(1).toLowerCase());
      setShowSlashPalette(true);
      setPaletteIndex(0);
    } else {
      setShowSlashPalette(false);
    }
  }, []);

  // Filtered commands for the slash palette
  const filteredCommands = useMemo(
    () => showSlashPalette
      ? commands.filter(c => c.name.toLowerCase().startsWith(slashQuery))
      : [],
    [showSlashPalette, commands, slashQuery]
  );

  const selectCommand = useCallback((cmd: SlashCommand) => {
    setShowSlashPalette(false);
    setValue('');
    if (textareaRef.current) textareaRef.current.style.height = 'auto';
    if (cmd.builtin) {
      if (cmd.name === 'think') {
        setThinking(prev => !prev);
      }
      textareaRef.current?.focus();
      return;
    }
    if (cmd.chatType) {
      setChatType(cmd.chatType);
    }
    textareaRef.current?.focus();
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

  const handlePaste = useCallback((e: React.ClipboardEvent) => {
    const items = e.clipboardData?.items;
    if (!items) return;
    const pastedFiles: File[] = [];
    for (let i = 0; i < items.length; i++) {
      const item = items[i];
      if (item.kind === 'file') {
        const file = item.getAsFile();
        if (file) pastedFiles.push(file);
      }
    }
    if (pastedFiles.length > 0) {
      e.preventDefault();
      setFiles((prev) => [...prev, ...pastedFiles]);
    }
  }, []);

  const startListening = useCallback(() => {
    if (!speechSupported) return;
    const SpeechRecognition = (window as any).webkitSpeechRecognition || (window as any).SpeechRecognition;
    const recognition = new SpeechRecognition();
    recognition.continuous = true;
    recognition.interimResults = true;
    recognition.lang = 'en-US';

    recognition.onresult = (event: any) => {
      let interim = '';
      let final = '';
      for (let i = event.resultIndex; i < event.results.length; i++) {
        const transcript = event.results[i][0].transcript;
        if (event.results[i].isFinal) {
          final += transcript;
        } else {
          interim += transcript;
        }
      }
      if (final) {
        setValue((prev) => prev + final);
        setInterimText('');
      } else {
        setInterimText(interim);
      }
    };

    recognition.onerror = (event: any) => {
      console.error('[Soul] Speech recognition error:', event.error, event.message);
      setIsListening(false);
      setInterimText('');
    };

    recognition.onend = () => {
      setIsListening(false);
      setInterimText('');
    };

    recognitionRef.current = recognition;
    recognition.start();
    setIsListening(true);
  }, [speechSupported]);

  const stopListening = useCallback(() => {
    if (recognitionRef.current) {
      recognitionRef.current.stop();
      recognitionRef.current = null;
    }
    setIsListening(false);
    setInterimText('');
  }, []);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
      // Slash palette navigation
      if (showSlashPalette && filteredCommands.length > 0) {
        if (e.key === 'ArrowDown') {
          e.preventDefault();
          setPaletteIndex(i => Math.min(i + 1, filteredCommands.length - 1));
          return;
        }
        if (e.key === 'ArrowUp') {
          e.preventDefault();
          setPaletteIndex(i => Math.max(i - 1, 0));
          return;
        }
        if (e.key === 'Enter' || e.key === 'Tab') {
          e.preventDefault();
          if (filteredCommands[paletteIndex]) selectCommand(filteredCommands[paletteIndex]);
          return;
        }
        if (e.key === 'Escape') {
          e.preventDefault();
          setShowSlashPalette(false);
          return;
        }
      }
      if (e.key === 'Escape' && isListening) {
        e.preventDefault();
        stopListening();
        return;
      }
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        if (isListening) stopListening();
        handleSend();
      }
    },
    [handleSend, isListening, stopListening, showSlashPalette, filteredCommands, paletteIndex, selectCommand],
  );

  return (
    <div className="px-5 py-4">
      <div className="relative">
        {/* Slash command palette — positioned above the input box */}
        {showSlashPalette && filteredCommands.length > 0 && (
          <div className="absolute bottom-full left-0 right-0 mb-2 bg-surface border border-border-default rounded-xl shadow-xl z-50 overflow-hidden max-h-64 overflow-y-auto">
            <div className="px-3 py-1.5 text-[10px] font-mono uppercase tracking-widest text-fg-muted border-b border-border-subtle">
              Commands
            </div>
            {filteredCommands.map((cmd, i) => (
              <button
                key={cmd.name}
                type="button"
                onMouseDown={(e) => { e.preventDefault(); selectCommand(cmd); }}
                className={`w-full flex items-center gap-3 px-3 py-2 text-left transition-colors cursor-pointer ${
                  i === paletteIndex ? 'bg-elevated' : 'hover:bg-elevated/50'
                }`}
              >
                <span className="font-mono text-soul text-sm">/{cmd.name}</span>
                <span className="text-xs text-fg-muted flex-1">{cmd.description}</span>
              </button>
            ))}
          </div>
        )}
        <div className="bg-elevated border border-border-default rounded-2xl overflow-hidden shadow-lg shadow-black/20" onPaste={handlePaste}>
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

        {/* Interim speech text */}
        {isListening && interimText && (
          <div className="px-4 pb-1 text-fg-muted text-sm italic truncate">{interimText}</div>
        )}

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
            <optgroup label="Modes">
              {CHAT_TYPES.filter(t => t.group === 'mode').map((t) => (
                <option key={t.value} value={t.value}>{t.label}</option>
              ))}
            </optgroup>
            <optgroup label="Workflows">
              {CHAT_TYPES.filter(t => t.group === 'skill').map((t) => (
                <option key={t.value} value={t.value}>{t.label}</option>
              ))}
            </optgroup>
          </select>

          {/* Extended thinking toggle */}
          {model.includes('opus') && (
            <button
              type="button"
              onClick={() => setThinking(!thinking)}
              className={`w-7 h-7 flex items-center justify-center rounded transition-colors cursor-pointer ${
                thinking ? 'bg-soul/20 text-soul' : 'text-fg-muted hover:text-fg hover:bg-elevated'
              }`}
              title={thinking ? 'Extended thinking ON' : 'Extended thinking OFF'}
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <path d="M8 2a5 5 0 0 1 3 9v1.5a1.5 1.5 0 0 1-1.5 1.5h-3A1.5 1.5 0 0 1 5 12.5V11a5 5 0 0 1 3-9z" />
                <path d="M6 14.5h4" />
              </svg>
            </button>
          )}

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

          {/* Send or Mic button */}
          {!value.trim() && !disabled && speechSupported ? (
            <button
              type="button"
              onClick={isListening ? stopListening : startListening}
              className={`w-8 h-8 rounded-full flex items-center justify-center transition-colors shrink-0 cursor-pointer ${
                isListening
                  ? 'bg-stage-blocked text-white animate-soul-pulse'
                  : 'bg-elevated text-fg-muted hover:text-fg hover:bg-overlay'
              }`}
              title={isListening ? 'Stop listening' : 'Voice input'}
            >
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <rect x="5" y="1" width="6" height="8" rx="3" />
                <path d="M3 7v1a5 5 0 0 0 10 0V7" />
                <path d="M8 13v2" />
              </svg>
            </button>
          ) : (
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
          )}
        </div>
        </div>
      </div>
    </div>
  );
}
