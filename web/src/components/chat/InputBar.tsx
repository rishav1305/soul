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

interface ContextChip {
  label: string;
  onInject: () => void;
  onDismiss: () => void;
}

interface InputBarProps {
  onSend: (message: string, options?: SendOptions) => void;
  disabled: boolean;
  isStreaming?: boolean;
  onStop?: () => void;
  contextChip?: string | null;
  onInjectContext?: () => void;
  onDismissChip?: () => void;
  contextChips?: ContextChip[];
  droppedFiles?: File[];
  onDroppedFilesConsumed?: () => void;
}

const CHAT_TYPES = [
  { value: 'Chat', label: 'Chat', group: 'mode' },
  { value: 'Code', label: 'Code', group: 'mode' },
  { value: 'Architect', label: 'Architect', group: 'mode' },
  { value: 'Debug', label: 'Debug', group: 'skill' },
  { value: 'Review', label: 'Review', group: 'skill' },
  { value: 'TDD', label: 'TDD', group: 'skill' },
  { value: 'Brainstorm', label: 'Brainstorm', group: 'skill' },
  { value: 'Clarify', label: 'Clarify', group: 'skill' },
] as const;

const PREFS_KEY = 'soul-chat-prefs';

interface ChatPrefs {
  model?: string;
  chatType?: string;
  thinking?: boolean;
  disabledTools?: string[];
}

function loadPrefs(): ChatPrefs {
  try {
    const raw = localStorage.getItem(PREFS_KEY);
    return raw ? JSON.parse(raw) : {};
  } catch { return {}; }
}

function savePrefs(partial: Partial<ChatPrefs>): void {
  try {
    const prev = loadPrefs();
    localStorage.setItem(PREFS_KEY, JSON.stringify({ ...prev, ...partial }));
  } catch { /* ignore */ }
}

export default function InputBar({ onSend, disabled, isStreaming = false, onStop, contextChip, onInjectContext, onDismissChip, contextChips = [], droppedFiles, onDroppedFilesConsumed }: InputBarProps) {
  const prefs = useRef(loadPrefs()).current;
  const [value, setValue] = useState('');
  const [model, _setModel] = useState<string>(prefs.model ?? '');
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [chatType, _setChatType] = useState<string>(prefs.chatType ?? 'Chat');
  const [thinking, _setThinking] = useState(prefs.thinking ?? true);
  const [tools, setTools] = useState<ToolInfo[]>([]);
  const [disabledTools, _setDisabledTools] = useState<Set<string>>(new Set(prefs.disabledTools ?? []));

  const setModel = useCallback((v: string) => { _setModel(v); savePrefs({ model: v }); }, []);
  const setChatType = useCallback((v: string) => { _setChatType(v); savePrefs({ chatType: v }); }, []);
  const setThinking = useCallback((v: boolean | ((prev: boolean) => boolean)) => {
    _setThinking((prev) => {
      const next = typeof v === 'function' ? v(prev) : v;
      savePrefs({ thinking: next });
      return next;
    });
  }, []);
  const setDisabledTools = useCallback((v: Set<string> | ((prev: Set<string>) => Set<string>)) => {
    _setDisabledTools((prev) => {
      const next = typeof v === 'function' ? v(prev) : v;
      savePrefs({ disabledTools: Array.from(next) });
      return next;
    });
  }, []);
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

  // Fetch models on mount — use saved preference if valid, otherwise first model
  useEffect(() => {
    fetch('/api/models')
      .then((r) => r.json())
      .then((data: ModelInfo[]) => {
        setModels(data);
        if (data.length > 0) {
          const saved = loadPrefs().model;
          const valid = saved && data.some((m) => m.id === saved);
          setModel(valid ? saved! : data[0].id);
        }
      })
      .catch(() => {});
  }, [setModel]);

  // Fetch tools on mount
  useEffect(() => {
    fetch('/api/tools')
      .then((r) => r.json())
      .then((data: ToolInfo[]) => {
        if (Array.isArray(data)) setTools(data);
      })
      .catch(() => {});
  }, []);

  // Sticky focus — re-focus textarea when clicks land outside other inputs
  useEffect(() => {
    textareaRef.current?.focus();
    const handler = (e: MouseEvent) => {
      const target = e.target as HTMLElement;
      // Don't steal focus from other inputs, buttons, selects, or textareas
      if (target.closest('input, select, textarea, button, [role="button"], [contenteditable]')) return;
      setTimeout(() => textareaRef.current?.focus(), 0);
    };
    document.addEventListener('click', handler);
    return () => document.removeEventListener('click', handler);
  }, []);

  useEffect(() => {
    if (droppedFiles && droppedFiles.length > 0) {
      setFiles((prev) => [...prev, ...droppedFiles]);
      onDroppedFilesConsumed?.();
    }
  }, [droppedFiles, onDroppedFilesConsumed]);

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
    let trimmed = value.trim();
    if (!trimmed || disabled) return;

    // Detect inline /command at start of message (e.g. "/brainstorm Add stocks product")
    let effectiveChatType = chatType;
    if (trimmed.startsWith('/')) {
      const spaceIdx = trimmed.indexOf(' ');
      const cmdName = spaceIdx > 0 ? trimmed.slice(1, spaceIdx) : trimmed.slice(1);
      const matched = commands.find(c => c.name.toLowerCase() === cmdName.toLowerCase());
      if (matched) {
        if (matched.chatType) {
          effectiveChatType = matched.chatType;
          setChatType(matched.chatType);
        }
        if (matched.builtin && matched.name === 'think') {
          setThinking(prev => !prev);
        }
        trimmed = spaceIdx > 0 ? trimmed.slice(spaceIdx + 1).trim() : '';
        if (!trimmed) {
          // Command-only (no message) — just switch mode
          setValue('');
          if (textareaRef.current) textareaRef.current.style.height = 'auto';
          setTimeout(() => textareaRef.current?.focus(), 0);
          return;
        }
      }
    }

    const options: SendOptions = {};
    if (model) options.model = model;
    if (effectiveChatType !== 'Chat') options.chatType = effectiveChatType.toLowerCase();
    if (disabledTools.size > 0) options.disabledTools = Array.from(disabledTools);
    if (thinking) options.thinking = true;
    onSend(trimmed, Object.keys(options).length > 0 ? options : undefined);
    setValue('');
    setFiles([]);
    if (textareaRef.current) textareaRef.current.style.height = 'auto';
    setTimeout(() => textareaRef.current?.focus(), 0);
  }, [value, disabled, model, chatType, disabledTools, thinking, onSend, commands, setChatType, setThinking]);

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

  const selectCommand = useCallback((cmd: SlashCommand, inlineText?: string) => {
    setShowSlashPalette(false);
    if (textareaRef.current) textareaRef.current.style.height = 'auto';
    if (cmd.builtin) {
      if (cmd.name === 'think') {
        setThinking(prev => !prev);
      }
      setValue(inlineText ?? '');
      textareaRef.current?.focus();
      return;
    }
    if (cmd.chatType) {
      setChatType(cmd.chatType);
    }
    // If there's inline text after the command, keep it as the message
    setValue(inlineText ?? '');
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
      if (e.key === 'Escape' && isStreaming && onStop) {
        e.preventDefault();
        onStop();
        return;
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
    [handleSend, isListening, stopListening, isStreaming, onStop, showSlashPalette, filteredCommands, paletteIndex, selectCommand],
  );

  const placeholder = (chatType === 'Brainstorm' || chatType === 'Clarify')
    ? 'Describe what you want to build…'
    : 'Message Soul...';

  return (
    <div className="panel-container px-5 py-4">
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
        {/* Context injection chip (single) */}
        {contextChip && (
          <div className="flex items-center gap-2 px-4 pt-2.5">
            <button
              type="button"
              onClick={onInjectContext}
              className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-soul/10 border border-soul/30 text-soul text-[11px] font-display font-semibold hover:bg-soul/20 transition-colors cursor-pointer"
            >
              <svg width="11" height="11" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M1 4v5h5" /><path d="M15 12V7h-5" />
                <path d="M14.5 7A6 6 0 0 0 3 5.5" /><path d="M1.5 9A6 6 0 0 0 13 10.5" />
              </svg>
              {contextChip} context — inject?
            </button>
            <button
              type="button"
              onClick={onDismissChip}
              className="text-fg-muted hover:text-fg text-xs leading-none cursor-pointer"
              title="Dismiss"
            >
              ×
            </button>
          </div>
        )}
        {/* Active mode badge */}
        {chatType !== 'Chat' && (
          <div className="flex items-center gap-2 px-4 pt-3">
            <span className="inline-flex items-center gap-1.5 bg-soul/10 border border-soul/30 rounded-full px-2.5 py-0.5 text-[11px] font-mono text-soul">
              /{chatType.toLowerCase()}
              <button
                type="button"
                onClick={() => setChatType('Chat')}
                className="text-soul/50 hover:text-soul cursor-pointer"
                title="Reset to Chat mode"
              >
                ×
              </button>
            </span>
          </div>
        )}
        {/* Context chips (array) */}
        {contextChips.length > 0 && (
          <div className="flex flex-wrap gap-2 px-4 pt-3">
            {contextChips.map((chip, i) => (
              <span
                key={i}
                className="inline-flex items-center gap-1.5 bg-soul/10 border border-soul/20 rounded-lg px-2.5 py-1 text-xs text-soul"
              >
                <button
                  type="button"
                  onClick={chip.onInject}
                  className="hover:underline cursor-pointer truncate max-w-[200px]"
                >
                  {chip.label}
                </button>
                <button
                  type="button"
                  onClick={chip.onDismiss}
                  className="text-soul/60 hover:text-soul ml-0.5 cursor-pointer"
                >
                  ×
                </button>
              </span>
            ))}
          </div>
        )}
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
          placeholder={placeholder}
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
                    {m.name}
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
              className={`h-7 flex items-center gap-1 px-1.5 rounded transition-colors cursor-pointer ${
                thinking ? 'bg-soul/20 text-soul' : 'text-fg-secondary hover:text-fg hover:bg-elevated'
              }`}
              title={thinking ? 'Extended thinking ON' : 'Extended thinking OFF'}
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <path d="M8 2a5 5 0 0 1 3 9v1.5a1.5 1.5 0 0 1-1.5 1.5h-3A1.5 1.5 0 0 1 5 12.5V11a5 5 0 0 1 3-9z" />
                <path d="M6 14.5h4" />
              </svg>
              <span className="rp-label-view text-[10px] font-mono">Think</span>
            </button>
          )}

          {/* Tool permissions */}
          <div className="relative" ref={toolPopoverRef}>
            <button
              type="button"
              onClick={() => setShowToolPopover(!showToolPopover)}
              className="relative h-7 flex items-center gap-1 px-1.5 rounded hover:bg-elevated text-fg-secondary hover:text-fg transition-colors cursor-pointer"
              title="Tool permissions"
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <path d="M6 3h4l2 2v2l-5 5-5-5V5l2-2z" />
                <circle cx="8" cy="6" r="1" />
              </svg>
              <span className="rp-label-view text-[10px] font-mono">Tools</span>
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
            className="h-7 flex items-center gap-1 px-1.5 rounded hover:bg-elevated text-fg-secondary hover:text-fg transition-colors cursor-pointer"
            title="Attach file"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <path d="M13.5 7.5l-5.8 5.8a3.5 3.5 0 0 1-5-5l5.8-5.8a2.3 2.3 0 0 1 3.3 3.3L6 11.6a1.2 1.2 0 0 1-1.7-1.7L9.5 4.7" />
            </svg>
            <span className="rp-label-action text-[10px] font-mono">File</span>
          </button>
          <input ref={fileInputRef} type="file" multiple className="hidden" onChange={handleFileSelect} />

          {/* Image attach */}
          <button
            type="button"
            onClick={() => imageInputRef.current?.click()}
            className="h-7 flex items-center gap-1 px-1.5 rounded hover:bg-elevated text-fg-secondary hover:text-fg transition-colors cursor-pointer"
            title="Attach image"
          >
            <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
              <rect x="2" y="2" width="12" height="12" rx="2" />
              <circle cx="5.5" cy="5.5" r="1.5" />
              <path d="M14 10l-3-3-7 7" />
            </svg>
            <span className="rp-label-action text-[10px] font-mono">Image</span>
          </button>
          <input ref={imageInputRef} type="file" accept="image/*" multiple className="hidden" onChange={handleFileSelect} />

          {/* Spacer */}
          <div className="flex-1" />

          {/* Stop / Send / Mic button */}
          {isStreaming ? (
            <button
              type="button"
              onClick={() => { onStop(); setTimeout(() => textareaRef.current?.focus(), 0); }}
              className="h-8 rounded-full flex items-center gap-1.5 px-3 bg-stage-blocked/15 text-stage-blocked hover:bg-stage-blocked/25 transition-colors shrink-0 cursor-pointer"
              title="Stop generating (Esc)"
            >
              <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
                <rect x="3" y="3" width="10" height="10" rx="1.5" />
              </svg>
              <span className="text-[10px] font-mono">Stop</span>
            </button>
          ) : !value.trim() && !disabled && speechSupported ? (
            <button
              type="button"
              onClick={isListening ? stopListening : startListening}
              className={`h-8 rounded-full flex items-center gap-1.5 px-3 transition-colors shrink-0 cursor-pointer ${
                isListening
                  ? 'bg-stage-blocked text-white animate-soul-pulse'
                  : 'bg-soul/15 text-soul hover:bg-soul/25'
              }`}
              title={isListening ? 'Stop listening' : 'Voice input'}
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <rect x="5" y="1" width="6" height="8" rx="3" />
                <path d="M3 7v1a5 5 0 0 0 10 0V7" />
                <path d="M8 13v2" />
              </svg>
              {isListening && <span className="text-[10px] font-mono">Stop</span>}
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
