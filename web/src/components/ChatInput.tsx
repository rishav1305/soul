import { useState, useCallback, useRef, useEffect, forwardRef, useImperativeHandle } from 'react';
import type { KeyboardEvent, ChangeEvent, ClipboardEvent, DragEvent } from 'react';
import type { ChatInputProps, ChatAttachment, ChatProduct, ChatMode, ThinkingType, ThinkingConfig } from '../lib/types';
import { CommandPalette } from './CommandPalette';
import type { SlashCommand } from './CommandPalette';
import { ThinkingToggle } from './ThinkingToggle';
import { useModels } from '../hooks/useModels';

function shortModelName(name: string): string {
  return name.replace(/^Claude\s+/i, '');
}

function fileToAttachment(file: File): Promise<ChatAttachment> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => {
      const result = reader.result as string;
      const base64 = result.split(',')[1] ?? '';
      resolve({
        name: file.name,
        mediaType: file.type,
        data: base64,
        preview: file.type.startsWith('image/') ? result : undefined,
      });
    };
    reader.onerror = reject;
    reader.readAsDataURL(file);
  });
}

const MAX_ATTACHMENTS = 5;
const MAX_FILE_SIZE = 10 * 1024 * 1024; // 10MB

const SLASH_COMMANDS: SlashCommand[] = [
  { name: 'code', description: 'Code generation mode' },
  { name: 'architect', description: 'Architecture discussion' },
  { name: 'brainstorm', description: 'Brainstorm ideas' },
  { name: 'review', description: 'Code review mode' },
  { name: 'debug', description: 'Debug an issue' },
];

const CHAT_MODES: { id: ChatMode; label: string }[] = [
  { id: 'chat', label: 'Chat' },
  { id: 'code', label: 'Code' },
  { id: 'architect', label: 'Architect' },
  { id: 'brainstorm', label: 'Brainstorm' },
];

const PRODUCTS: { id: ChatProduct; name: string; icon: string; group?: string }[] = [
  // Core
  { id: 'tasks', name: 'Tasks', icon: '☑', group: 'Core' },
  { id: 'tutor', name: 'Tutor', icon: '🎓', group: 'Core' },
  { id: 'projects', name: 'Projects', icon: '🔧', group: 'Core' },
  { id: 'observe', name: 'Observe', icon: '📊', group: 'Core' },
  // Smart Agents
  { id: 'scout', name: 'Scout', icon: '🎯', group: 'Smart Agents' },
  { id: 'sentinel', name: 'Sentinel', icon: '🛡', group: 'Smart Agents' },
  { id: 'mesh', name: 'Mesh', icon: '🔗', group: 'Smart Agents' },
  { id: 'bench', name: 'Bench', icon: '📈', group: 'Smart Agents' },
  // Quality
  { id: 'compliance', name: 'Compliance', icon: '✓', group: 'Quality' },
  { id: 'qa', name: 'QA', icon: '🔍', group: 'Quality' },
  { id: 'analytics', name: 'Analytics', icon: '📉', group: 'Quality' },
  // Infrastructure
  { id: 'devops', name: 'DevOps', icon: '⚙', group: 'Infrastructure' },
  { id: 'dba', name: 'DBA', icon: '🗄', group: 'Infrastructure' },
  { id: 'migrate', name: 'Migrate', icon: '↗', group: 'Infrastructure' },
  // Data
  { id: 'dataeng', name: 'DataEng', icon: '🔄', group: 'Data' },
  { id: 'costops', name: 'CostOps', icon: '💰', group: 'Data' },
  { id: 'viz', name: 'Viz', icon: '📊', group: 'Data' },
  // Documentation
  { id: 'docs', name: 'Docs', icon: '📄', group: 'Documentation' },
  { id: 'api', name: 'API', icon: '🔌', group: 'Documentation' },
];

export interface ChatInputHandle {
  focus: () => void;
}

interface ChatInputExtendedProps extends ChatInputProps {
  activeProduct?: ChatProduct;
  onSetProduct?: (product: ChatProduct) => void;
}

const SpeechRecognition = (typeof window !== 'undefined')
  ? ((window as any).SpeechRecognition || (window as any).webkitSpeechRecognition) as (new () => SpeechRecognitionInstance) | undefined
  : undefined;

interface SpeechRecognitionResultItem {
  readonly transcript: string;
  readonly confidence: number;
}

interface SpeechRecognitionResult {
  readonly length: number;
  readonly isFinal: boolean;
  [index: number]: SpeechRecognitionResultItem;
}

interface SpeechRecognitionResultList {
  readonly length: number;
  [index: number]: SpeechRecognitionResult;
}

interface SpeechRecognitionEvent {
  readonly results: SpeechRecognitionResultList;
  readonly resultIndex: number;
}

interface SpeechRecognitionErrorEvent {
  readonly error: string;
  readonly message: string;
}

interface SpeechRecognitionInstance {
  continuous: boolean;
  interimResults: boolean;
  lang: string;
  start(): void;
  stop(): void;
  onresult: ((event: SpeechRecognitionEvent) => void) | null;
  onerror: ((event: SpeechRecognitionErrorEvent) => void) | null;
  onend: (() => void) | null;
}

export const ChatInput = forwardRef<ChatInputHandle, ChatInputExtendedProps>(function ChatInput({ onSend, onStop, disabled, isStreaming, activeProduct, onSetProduct }, ref) {
  const [value, setValue] = useState('');
  const { models } = useModels();
  const [selectedModel, setSelectedModel] = useState<string>('');

  // API list is authoritative: use stored preference only if it is still valid,
  // otherwise fall back to the first model the server advertises.
  useEffect(() => {
    if (models.length === 0) return;
    const stored = localStorage.getItem('soul-model');
    const resolved = stored && models.some(m => m.id === stored) ? stored : (models[0]?.id ?? '');
    setSelectedModel(resolved);
    localStorage.setItem('soul-model', resolved);
  }, [models]);
  // Haiku models don't support extended thinking — default to disabled
  const isHaiku = selectedModel.includes('haiku');
  const [thinkingType, setThinkingType] = useState<ThinkingType>(isHaiku ? 'disabled' : 'adaptive');
  const [isListening, setIsListening] = useState(false);
  const [attachments, setAttachments] = useState<ChatAttachment[]>([]);
  const [isDragging, setIsDragging] = useState(false);
  const [chatMode, setChatMode] = useState<ChatMode>('chat');
  const [showCodeInput, setShowCodeInput] = useState(false);
  const [codeSnippet, setCodeSnippet] = useState('');
  const [showModeMenu, setShowModeMenu] = useState(false);
  const [showProductMenu, setShowProductMenu] = useState(false);
  const [showModelMenu, setShowModelMenu] = useState(false);
  const [showSettingsMenu, setShowSettingsMenu] = useState(false);
  const productMenuRef = useRef<HTMLDivElement>(null);
  const modeMenuRef = useRef<HTMLDivElement>(null);
  const modelMenuRef = useRef<HTMLDivElement>(null);
  const settingsMenuRef = useRef<HTMLDivElement>(null);
  const recognitionRef = useRef<SpeechRecognitionInstance | null>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
  const cameraInputRef = useRef<HTMLInputElement>(null);
  const hasSpeech = !!SpeechRecognition;

  useImperativeHandle(ref, () => ({
    focus: () => textareaRef.current?.focus(),
  }), []);

  // Reset thinking to disabled when switching to a Haiku model
  useEffect(() => {
    if (isHaiku && thinkingType !== 'disabled') {
      setThinkingType('disabled');
    }
  }, [isHaiku]); // eslint-disable-line react-hooks/exhaustive-deps

  // Close product menu on outside click.
  useEffect(() => {
    if (!showProductMenu) return;
    const handler = (e: PointerEvent) => {
      if (productMenuRef.current && !productMenuRef.current.contains(e.target as Node)) {
        setShowProductMenu(false);
      }
    };
    document.addEventListener('pointerdown', handler);
    return () => document.removeEventListener('pointerdown', handler);
  }, [showProductMenu]);

  // Close mobile mode menu on outside click.
  useEffect(() => {
    if (!showModeMenu) return;
    const handler = (e: PointerEvent) => {
      if (modeMenuRef.current && !modeMenuRef.current.contains(e.target as Node)) {
        setShowModeMenu(false);
      }
    };
    document.addEventListener('pointerdown', handler);
    return () => document.removeEventListener('pointerdown', handler);
  }, [showModeMenu]);

  // Close model/thinking menu on outside click.
  useEffect(() => {
    if (!showModelMenu) return;
    const handler = (e: PointerEvent) => {
      if (modelMenuRef.current && !modelMenuRef.current.contains(e.target as Node)) {
        setShowModelMenu(false);
      }
    };
    document.addEventListener('pointerdown', handler);
    return () => document.removeEventListener('pointerdown', handler);
  }, [showModelMenu]);

  // Close settings menu on outside click.
  useEffect(() => {
    if (!showSettingsMenu) return;
    const handler = (e: PointerEvent) => {
      if (settingsMenuRef.current && !settingsMenuRef.current.contains(e.target as Node)) {
        setShowSettingsMenu(false);
      }
    };
    document.addEventListener('pointerdown', handler);
    return () => document.removeEventListener('pointerdown', handler);
  }, [showSettingsMenu]);

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

  const addFiles = useCallback(async (files: File[]) => {
    const valid = files.filter(f => f.size <= MAX_FILE_SIZE && f.type.startsWith('image/'));
    if (valid.length === 0) return;
    const newAtts = await Promise.all(valid.slice(0, MAX_ATTACHMENTS - attachments.length).map(fileToAttachment));
    setAttachments(prev => [...prev, ...newAtts].slice(0, MAX_ATTACHMENTS));
  }, [attachments.length]);

  const removeAttachment = useCallback((idx: number) => {
    setAttachments(prev => prev.filter((_, i) => i !== idx));
  }, []);

  const handlePaste = useCallback((e: ClipboardEvent<HTMLElement>) => {
    const files = Array.from(e.clipboardData.files);
    if (files.length > 0) {
      e.preventDefault();
      addFiles(files);
    }
  }, [addFiles]);

  const handleDragOver = useCallback((e: DragEvent) => {
    e.preventDefault();
    setIsDragging(true);
  }, []);

  const handleDragLeave = useCallback((e: DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
  }, []);

  const handleDrop = useCallback((e: DragEvent) => {
    e.preventDefault();
    setIsDragging(false);
    const files = Array.from(e.dataTransfer.files);
    addFiles(files);
  }, [addFiles]);

  const showPalette = value.startsWith('/') && !value.includes(' ') && value.length > 0;
  const paletteFilter = value.slice(1);

  const handleSlashSelect = useCallback((cmd: SlashCommand) => {
    setValue(`/${cmd.name} `);
    textareaRef.current?.focus();
  }, []);

  const handlePaletteClose = useCallback(() => {
    setValue('');
    textareaRef.current?.focus();
  }, []);

  const handleChange = useCallback(
    (e: ChangeEvent<HTMLTextAreaElement>) => {
      setValue(e.target.value);
      resizeTextarea();
    },
    [resizeTextarea],
  );

  const handleModelChange = useCallback((e: ChangeEvent<HTMLSelectElement>) => {
    setSelectedModel(e.target.value);
    localStorage.setItem('soul-model', e.target.value);
  }, []);

  const toggleListening = useCallback(() => {
    if (!SpeechRecognition) return;

    if (isListening && recognitionRef.current) {
      recognitionRef.current.stop();
      setIsListening(false);
      return;
    }

    const recognition = new SpeechRecognition();
    recognition.continuous = true;
    recognition.interimResults = true;
    recognition.lang = 'en-US';

    recognition.onresult = (event: SpeechRecognitionEvent) => {
      let transcript = '';
      for (let i = 0; i < event.results.length; i++) {
        transcript += event.results[i]?.[0]?.transcript ?? '';
      }
      setValue(prev => {
        // Replace from where speech started, keep any text typed before
        const prefix = prev.split('\u200B')[0] || '';
        return prefix + transcript;
      });
      resizeTextarea();
    };

    recognition.onerror = () => {
      setIsListening(false);
      recognitionRef.current = null;
    };

    recognition.onend = () => {
      setIsListening(false);
      recognitionRef.current = null;
    };

    recognitionRef.current = recognition;
    recognition.start();
    setIsListening(true);
  }, [isListening, resizeTextarea]);

  const handleSend = useCallback(() => {
    const trimmed = value.trim();
    const codeTrimmed = codeSnippet.trim();
    if (!trimmed && !codeTrimmed && attachments.length === 0) return;
    if (disabled) return;
    const opts: { model?: string; thinking?: ThinkingConfig; attachments?: ChatAttachment[] } = {};
    if (selectedModel) opts.model = selectedModel;
    if (thinkingType !== 'disabled') {
      const model = models.find(m => m.id === selectedModel);
      const maxTok = model?.max_tokens ?? 64000;
      opts.thinking = {
        type: thinkingType,
        ...(thinkingType === 'enabled' ? { budget_tokens: maxTok - 1024 } : {}),
      };
    }
    if (attachments.length > 0) opts.attachments = attachments;
    let body = trimmed || '(attached image)';
    if (codeTrimmed) {
      body = body === '(attached image)' ? `\`\`\`python\n${codeTrimmed}\n\`\`\`` : `${body}\n\n\`\`\`python\n${codeTrimmed}\n\`\`\``;
    }
    const content = chatMode !== 'chat' ? `/${chatMode} ${body}` : body;
    onSend(content, Object.keys(opts).length > 0 ? opts : undefined);
    setValue('');
    setCodeSnippet('');
    setShowCodeInput(false);
    setAttachments([]);
    requestAnimationFrame(() => {
      const ta = textareaRef.current;
      if (ta) {
        ta.style.height = 'auto';
      }
    });
  }, [value, codeSnippet, disabled, onSend, selectedModel, thinkingType, models, attachments, chatMode]);

  const handleKeyDown = useCallback(
    (e: KeyboardEvent<HTMLTextAreaElement>) => {
      if (e.key === 'Escape' && isStreaming) {
        e.preventDefault();
        onStop();
        return;
      }
      // Let CommandPalette handle navigation keys when open
      if (showPalette && (e.key === 'ArrowUp' || e.key === 'ArrowDown' || e.key === 'Tab' || e.key === 'Enter' || e.key === 'Escape')) {
        e.preventDefault();
        return;
      }
      if (e.key === 'Enter' && !e.shiftKey) {
        e.preventDefault();
        handleSend();
      }
    },
    [handleSend, isStreaming, onStop, showPalette],
  );

  return (
    <div
      className={`relative px-3 md:px-5 py-3 md:py-4 safe-bottom ${isDragging ? 'ring-2 ring-soul/50 bg-soul/5 rounded-xl' : ''}`}
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
    >
      {isDragging && (
        <div className="absolute inset-0 z-10 flex items-center justify-center bg-deep/80 rounded-2xl pointer-events-none">
          <span className="text-soul text-sm font-medium">Drop images here</span>
        </div>
      )}
      <div className="relative">
        {/* Slash command palette — above input */}
        {showPalette && (
          <div className="absolute bottom-full left-0 right-0 mb-2 z-50">
            <CommandPalette
              commands={SLASH_COMMANDS}
              filter={paletteFilter}
              onSelect={handleSlashSelect}
              onClose={handlePaletteClose}
            />
          </div>
        )}
        {/* Elevated input card */}
        <div className="bg-elevated border border-border-default rounded-2xl shadow-lg shadow-black/20" onPaste={handlePaste}>
          {/* Attachment chips inside card */}
          {attachments.length > 0 && (
            <div data-testid="attachment-chips" className="flex gap-2 flex-wrap px-4 pt-3">
              {attachments.map((att, i) => (
                <span key={i} className="inline-flex items-center gap-1.5 bg-elevated rounded-lg px-2.5 py-1 text-xs text-fg-secondary border border-border-subtle">
                  {att.preview ? (
                    <img src={att.preview} alt={att.name} className="h-5 w-5 object-cover rounded" />
                  ) : (
                    <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5"><rect x="2" y="2" width="12" height="12" rx="2" /><circle cx="6" cy="6.5" r="1.5" /><path d="M2 11l3-3 2 2 3-3 4 4" /></svg>
                  )}
                  <span className="max-w-[120px] truncate">{att.name}</span>
                  <button
                    data-testid={`remove-attachment-${i}`}
                    type="button"
                    onClick={() => removeAttachment(i)}
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
            data-testid="chat-input"
            className="w-full bg-transparent px-4 pt-3 pb-2 text-fg placeholder:text-fg-muted resize-none overflow-y-hidden focus:outline-none disabled:opacity-50 disabled:cursor-not-allowed"
            placeholder={chatMode === 'brainstorm' ? 'Describe what you want to build...' : chatMode === 'architect' ? 'Describe the architecture...' : chatMode === 'code' ? 'Describe what to code...' : 'Message...'}
            rows={1}
            value={value}
            onChange={handleChange}
            onKeyDown={handleKeyDown}
            disabled={disabled}
          />

          {/* Code snippet area */}
          {showCodeInput && (
            <div className="mx-3 mb-1 rounded-lg border border-border-subtle bg-deep overflow-hidden">
              <div className="flex items-center justify-between px-2.5 py-1 bg-surface/50 border-b border-border-subtle">
                <span className="text-[10px] font-mono text-fg-muted uppercase tracking-wider">Python</span>
                <button
                  type="button"
                  onClick={() => { setShowCodeInput(false); setCodeSnippet(''); }}
                  className="text-fg-muted hover:text-fg cursor-pointer"
                  aria-label="Remove code snippet"
                >
                  <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round">
                    <line x1="4" y1="4" x2="12" y2="12" /><line x1="12" y1="4" x2="4" y2="12" />
                  </svg>
                </button>
              </div>
              <textarea
                data-testid="code-snippet-input"
                className="w-full bg-transparent px-3 py-2 text-[12px] sm:text-[13px] font-mono text-fg placeholder:text-fg-muted/50 resize-none focus:outline-none"
                placeholder="Paste or type Python code..."
                rows={4}
                value={codeSnippet}
                onChange={(e) => setCodeSnippet(e.target.value)}
                spellCheck={false}
              />
            </div>
          )}

          {/* Hidden file inputs */}
          <input
            ref={fileInputRef}
            type="file"
            accept="image/*"
            multiple
            className="hidden"
            onChange={(e) => {
              const files = Array.from(e.target.files || []);
              if (files.length > 0) addFiles(files);
              e.target.value = '';
            }}
          />
          <input
            ref={cameraInputRef}
            type="file"
            accept="image/*"
            capture="environment"
            className="hidden"
            onChange={(e) => {
              const files = Array.from(e.target.files || []);
              if (files.length > 0) addFiles(files);
              e.target.value = '';
            }}
          />

          {/* Toolbar */}
          <div className="flex flex-wrap items-center gap-1.5 sm:gap-2 px-3 py-2 sm:py-2.5 border-t border-border-subtle">
            {/* Product selector */}
            <div ref={productMenuRef} className="relative">
              <button
                data-testid="product-selector-button"
                type="button"
                onClick={() => setShowProductMenu(prev => !prev)}
                className={`h-8 flex items-center gap-1.5 px-2 rounded-lg transition-colors cursor-pointer ${
                  activeProduct ? 'text-blue-400 bg-blue-500/15 hover:bg-blue-500/25' : 'text-fg-muted hover:text-fg hover:bg-elevated'
                }`}
                title="Select product context"
                aria-label="Select product context"
              >
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <path d="M2 4h12M4 8h8M6 12h4" />
                  <circle cx="13" cy="4" r="1.5" fill="currentColor" stroke="none" />
                  <circle cx="11" cy="8" r="1.5" fill="currentColor" stroke="none" />
                  <circle cx="9" cy="12" r="1.5" fill="currentColor" stroke="none" />
                </svg>
                <span
                  data-testid="product-badge"
                  className={`text-xs font-mono ${activeProduct ? 'text-soul' : 'text-fg-muted'}`}
                >
                  {activeProduct
                    ? (PRODUCTS.find(p => p.id === activeProduct)?.name ?? activeProduct)
                    : 'General'}
                </span>
              </button>
              {showProductMenu && (
                <div className="absolute bottom-full left-0 mb-1.5 z-50 bg-elevated border border-border-default rounded-xl shadow-xl shadow-black/30 py-1 min-w-[160px] max-h-[70vh] overflow-y-auto">
                  <button
                    data-testid="product-option-none"
                    type="button"
                    onClick={() => { onSetProduct?.(''); setShowProductMenu(false); }}
                    className={`w-full text-left flex items-center gap-2 px-3 py-1.5 text-xs transition-colors cursor-pointer ${
                      !activeProduct ? 'text-soul bg-soul/10' : 'text-fg-muted hover:text-fg hover:bg-elevated'
                    }`}
                  >
                    <span className="w-4 text-center">—</span>
                    <span>General</span>
                  </button>
                  {PRODUCTS.map((p, i) => {
                    const prev = i > 0 ? PRODUCTS[i - 1] : null;
                    const showGroup = p.group && (!prev || prev.group !== p.group);
                    return (
                      <div key={p.id}>
                        {showGroup && (
                          <>
                            <div className="border-t border-border-subtle my-1" />
                            <div className="px-3 pt-1 pb-0.5 text-[10px] text-fg-muted uppercase tracking-wider font-medium">{p.group}</div>
                          </>
                        )}
                        <button
                          data-testid={`product-option-${p.id}`}
                          type="button"
                          onClick={() => { onSetProduct?.(p.id); setShowProductMenu(false); }}
                          className={`w-full text-left flex items-center gap-2 px-3 py-1.5 text-xs transition-colors cursor-pointer ${
                            activeProduct === p.id ? 'text-soul bg-soul/10' : 'text-fg-muted hover:text-fg hover:bg-elevated'
                          }`}
                        >
                          <span className={`w-1.5 h-1.5 rounded-full ${activeProduct === p.id ? 'bg-soul' : 'bg-fg-muted/40'}`} />
                          <span>{p.name}</span>
                        </button>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>

            {/* Desktop: chat mode buttons + model select + thinking toggle */}
            <div className="hidden sm:contents">
              <div data-testid="chat-mode-selector" className="flex relative items-center h-8 bg-surface rounded-lg px-0.5">
                {CHAT_MODES.map((mode) => (
                  <button
                    key={mode.id}
                    data-testid={`chat-mode-${mode.id}`}
                    type="button"
                    onClick={() => setChatMode(mode.id)}
                    className={`relative z-10 h-7 px-2.5 text-xs font-mono rounded-md transition-colors cursor-pointer ${
                      chatMode === mode.id ? 'text-fg font-semibold' : 'text-fg-muted hover:text-fg'
                    }`}
                  >
                    {chatMode === mode.id && <span className="absolute inset-0 bg-elevated shadow-sm rounded-md -z-10" />}
                    {mode.label}
                  </button>
                ))}
              </div>
              <select
                data-testid="model-selector"
                value={selectedModel}
                onChange={handleModelChange}
                className="soul-select"
              >
                {models.map(m => (
                  <option key={m.id} value={m.id}>{m.name}</option>
                ))}
              </select>
              <ThinkingToggle value={thinkingType} onChange={setThinkingType} disabled={isHaiku} />
            </div>

            {/* Mobile: single settings gear → popover with mode + model + thinking */}
            <div ref={settingsMenuRef} className="sm:hidden relative">
              <button
                data-testid="settings-btn"
                type="button"
                onClick={() => setShowSettingsMenu(prev => !prev)}
                className="w-8 h-8 flex items-center justify-center rounded-lg bg-surface text-fg-muted hover:text-fg transition-colors cursor-pointer"
                title="Chat settings"
              >
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <circle cx="8" cy="8" r="2" />
                  <path d="M6.6 1.2l-.3 1.5a5 5 0 00-1.6.9L3.3 3 1.9 4.5l1 1.3a5 5 0 000 1.8l-1 1.3L3.3 10.4l1.4-.6a5 5 0 001.6.9l.3 1.5h2.8l.3-1.5a5 5 0 001.6-.9l1.4.6 1.4-1.4-1-1.3a5 5 0 000-1.8l1-1.3L13.7 3l-1.4.6a5 5 0 00-1.6-.9l-.3-1.5z" />
                </svg>
              </button>
              {showSettingsMenu && (
                <div className="absolute bottom-full mb-1 left-0 z-50 bg-elevated border border-border-default rounded-xl shadow-xl shadow-black/30 py-1 min-w-[160px]">
                  <div className="px-3 pt-1.5 pb-1 text-[10px] text-fg-muted uppercase tracking-wider font-medium">Mode</div>
                  {CHAT_MODES.map(m => (
                    <button
                      key={m.id}
                      data-testid={`chat-mode-option-${m.id}`}
                      type="button"
                      onClick={() => setChatMode(m.id)}
                      className={`w-full text-left flex items-center gap-2 px-3 py-1.5 text-xs font-mono transition-colors cursor-pointer ${
                        chatMode === m.id ? 'text-soul bg-soul/10' : 'text-fg-muted hover:text-fg hover:bg-elevated'
                      }`}
                    >
                      <span className={`w-1.5 h-1.5 rounded-full ${chatMode === m.id ? 'bg-soul' : 'bg-fg-muted/40'}`} />
                      <span>{m.label}</span>
                    </button>
                  ))}
                  <div className="border-t border-border-subtle my-1" />
                  <div className="px-3 pt-1 pb-1 text-[10px] text-fg-muted uppercase tracking-wider font-medium">Model</div>
                  {models.map(m => (
                    <button
                      key={m.id}
                      data-testid={`model-option-${m.id}`}
                      type="button"
                      onClick={() => { setSelectedModel(m.id); localStorage.setItem('soul-model', m.id); }}
                      className={`w-full text-left flex items-center gap-2 px-3 py-1.5 text-xs font-mono transition-colors cursor-pointer ${
                        selectedModel === m.id ? 'text-soul bg-soul/10' : 'text-fg-muted hover:text-fg hover:bg-elevated'
                      }`}
                    >
                      <span className={`w-1.5 h-1.5 rounded-full ${selectedModel === m.id ? 'bg-soul' : 'bg-fg-muted/40'}`} />
                      <span>{shortModelName(m.name)}</span>
                    </button>
                  ))}
                  <div className="border-t border-border-subtle my-1" />
                  <div className="px-3 pt-1 pb-1 text-[10px] text-fg-muted uppercase tracking-wider font-medium">
                    Thinking{isHaiku && <span className="ml-1 text-zinc-600">(N/A)</span>}
                  </div>
                  {([
                    { type: 'disabled' as ThinkingType, label: 'Off' },
                    { type: 'adaptive' as ThinkingType, label: 'Auto' },
                    { type: 'enabled' as ThinkingType, label: 'Max' },
                  ]).map(t => (
                    <button
                      key={t.type}
                      data-testid={`thinking-option-${t.type}`}
                      type="button"
                      onClick={() => !isHaiku && setThinkingType(t.type)}
                      disabled={isHaiku && t.type !== 'disabled'}
                      className={`w-full text-left flex items-center gap-2 px-3 py-1.5 text-xs font-mono transition-colors ${
                        isHaiku && t.type !== 'disabled' ? 'text-zinc-700 cursor-not-allowed' :
                        thinkingType === t.type ? 'text-soul bg-soul/10 cursor-pointer' : 'text-fg-muted hover:text-fg hover:bg-elevated cursor-pointer'
                      }`}
                    >
                      <span className={`w-1.5 h-1.5 rounded-full ${thinkingType === t.type ? 'bg-soul' : 'bg-fg-muted/40'}`} />
                      <span>{t.label}</span>
                    </button>
                  ))}
                </div>
              )}
            </div>

            {/* Attach button */}
            <button
              data-testid="attach-button"
              type="button"
              onClick={() => fileInputRef.current?.click()}
              className="h-8 flex items-center gap-1.5 px-2 rounded-lg text-fg-secondary hover:text-fg hover:bg-elevated transition-colors cursor-pointer"
              aria-label="Attach file"
              title="Attach file"
            >
              <svg width="14" height="14" viewBox="0 0 20 20" fill="none">
                <path d="M17.5 9.5l-7.8 7.8a4.2 4.2 0 01-6-6l7.9-7.8a2.8 2.8 0 014 4L7.7 15.3a1.4 1.4 0 01-2-2l7-6.9" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
              </svg>
              <span className="hidden sm:inline text-xs">Attach</span>
            </button>

            {/* Code snippet button */}
            <button
              data-testid="code-snippet-button"
              type="button"
              onClick={() => setShowCodeInput(prev => !prev)}
              className={`h-8 flex items-center gap-1.5 px-2 rounded-lg transition-colors cursor-pointer ${
                showCodeInput ? 'text-soul bg-soul/15' : 'text-fg-secondary hover:text-fg hover:bg-elevated'
              }`}
              aria-label="Add code snippet"
              title="Add code snippet"
            >
              <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <polyline points="4,4 1,8 4,12" />
                <polyline points="12,4 15,8 12,12" />
                <line x1="10" y1="3" x2="6" y2="13" />
              </svg>
            </button>

            {/* Camera button — hidden on mobile (merged into attach) */}
            <button
              data-testid="camera-button"
              type="button"
              onClick={() => cameraInputRef.current?.click()}
              className="hidden sm:flex h-8 items-center gap-1.5 px-2 rounded-lg text-fg-secondary hover:text-fg hover:bg-elevated transition-colors cursor-pointer"
              aria-label="Take photo"
              title="Take photo"
            >
              <svg width="16" height="16" viewBox="0 0 20 20" fill="none">
                <path d="M7 3l-1.5 2H3a1 1 0 00-1 1v9a1 1 0 001 1h14a1 1 0 001-1V6a1 1 0 00-1-1h-2.5L13 3H7z" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
                <circle cx="10" cy="10.5" r="3" stroke="currentColor" strokeWidth="1.5" />
              </svg>
            </button>

            {/* Spacer */}
            <div className="flex-1" />

            {/* Stop / Send / Mic button */}
            {isStreaming ? (
              <button
                data-testid="stop-button"
                type="button"
                onClick={onStop}
                className="h-8 rounded-full flex items-center gap-1.5 px-3 bg-red-500/15 text-red-400 hover:bg-red-500/25 transition-colors shrink-0 cursor-pointer"
                title="Stop generating (Esc)"
              >
                <svg width="12" height="12" viewBox="0 0 16 16" fill="currentColor">
                  <rect x="3" y="3" width="10" height="10" rx="1.5" />
                </svg>
                <span className="text-xs font-mono">Stop</span>
              </button>
            ) : !value.trim() && attachments.length === 0 && !disabled && hasSpeech ? (
              <button
                data-testid="mic-button"
                type="button"
                onClick={toggleListening}
                className={`h-8 rounded-full flex items-center gap-1.5 px-3 transition-colors shrink-0 cursor-pointer ${
                  isListening
                    ? 'bg-red-500 text-white animate-soul-pulse'
                    : 'bg-soul/15 text-soul hover:bg-soul/25'
                }`}
                aria-label={isListening ? 'Stop recording' : 'Start recording'}
                title={isListening ? 'Stop recording' : 'Voice input'}
              >
                <svg width="14" height="14" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                  <rect x="5" y="1" width="6" height="8" rx="3" />
                  <path d="M3 7v1a5 5 0 0 0 10 0V7" />
                  <path d="M8 13v2" />
                </svg>
                {isListening && <span className="text-xs font-mono">Stop</span>}
              </button>
            ) : (
              <button
                data-testid="send-button"
                onClick={handleSend}
                disabled={disabled || (!value.trim() && attachments.length === 0)}
                className="w-10 h-10 sm:w-8 sm:h-8 bg-soul text-deep rounded-full flex items-center justify-center hover:bg-soul/85 disabled:opacity-20 disabled:cursor-not-allowed transition-colors shrink-0 cursor-pointer"
                title="Send"
                type="button"
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
});
