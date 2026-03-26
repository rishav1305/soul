import { useState, useCallback, useRef, useEffect } from 'react';
import SyntaxHighlighter from 'react-syntax-highlighter/dist/esm/prism-light';
import oneDark from 'react-syntax-highlighter/dist/esm/styles/prism/one-dark';
// Import only languages we actually use
import tsx from 'react-syntax-highlighter/dist/esm/languages/prism/tsx';
import typescript from 'react-syntax-highlighter/dist/esm/languages/prism/typescript';
import javascript from 'react-syntax-highlighter/dist/esm/languages/prism/javascript';
import go from 'react-syntax-highlighter/dist/esm/languages/prism/go';
import python from 'react-syntax-highlighter/dist/esm/languages/prism/python';
import bash from 'react-syntax-highlighter/dist/esm/languages/prism/bash';
import json from 'react-syntax-highlighter/dist/esm/languages/prism/json';
import yaml from 'react-syntax-highlighter/dist/esm/languages/prism/yaml';
import css from 'react-syntax-highlighter/dist/esm/languages/prism/css';
import sql from 'react-syntax-highlighter/dist/esm/languages/prism/sql';
import markdown from 'react-syntax-highlighter/dist/esm/languages/prism/markdown';
import rust from 'react-syntax-highlighter/dist/esm/languages/prism/rust';
import java from 'react-syntax-highlighter/dist/esm/languages/prism/java';
import docker from 'react-syntax-highlighter/dist/esm/languages/prism/docker';

SyntaxHighlighter.registerLanguage('tsx', tsx);
SyntaxHighlighter.registerLanguage('typescript', typescript);
SyntaxHighlighter.registerLanguage('javascript', javascript);
SyntaxHighlighter.registerLanguage('go', go);
SyntaxHighlighter.registerLanguage('python', python);
SyntaxHighlighter.registerLanguage('bash', bash);
SyntaxHighlighter.registerLanguage('json', json);
SyntaxHighlighter.registerLanguage('yaml', yaml);
SyntaxHighlighter.registerLanguage('css', css);
SyntaxHighlighter.registerLanguage('sql', sql);
SyntaxHighlighter.registerLanguage('markdown', markdown);
SyntaxHighlighter.registerLanguage('rust', rust);
SyntaxHighlighter.registerLanguage('java', java);
SyntaxHighlighter.registerLanguage('docker', docker);

// Aliases
SyntaxHighlighter.registerLanguage('ts', typescript);
SyntaxHighlighter.registerLanguage('js', javascript);
SyntaxHighlighter.registerLanguage('jsx', tsx);
SyntaxHighlighter.registerLanguage('py', python);
SyntaxHighlighter.registerLanguage('sh', bash);
SyntaxHighlighter.registerLanguage('shell', bash);
SyntaxHighlighter.registerLanguage('zsh', bash);
SyntaxHighlighter.registerLanguage('yml', yaml);
SyntaxHighlighter.registerLanguage('md', markdown);
SyntaxHighlighter.registerLanguage('rs', rust);
SyntaxHighlighter.registerLanguage('golang', go);
SyntaxHighlighter.registerLanguage('dockerfile', docker);

// Theme overrides for soul palette
const codeTheme = {
  ...oneDark,
  'pre[class*="language-"]': {
    ...oneDark['pre[class*="language-"]'],
    background: '#111118', // surface
    margin: 0,
    padding: '1rem',
    fontSize: '0.8rem',
    lineHeight: '1.5',
    borderRadius: 0,
  },
  'code[class*="language-"]': {
    ...oneDark['code[class*="language-"]'],
    background: 'none',
    fontSize: '0.8rem',
    lineHeight: '1.5',
  },
};

// Friendly language labels
const LANG_LABELS: Record<string, string> = {
  js: 'JavaScript', javascript: 'JavaScript',
  ts: 'TypeScript', typescript: 'TypeScript',
  tsx: 'TSX', jsx: 'JSX',
  go: 'Go', golang: 'Go',
  py: 'Python', python: 'Python',
  rs: 'Rust', rust: 'Rust',
  sh: 'Shell', bash: 'Bash', zsh: 'Zsh', shell: 'Shell',
  sql: 'SQL',
  css: 'CSS',
  json: 'JSON', yaml: 'YAML', yml: 'YAML',
  md: 'Markdown', markdown: 'Markdown',
  dockerfile: 'Dockerfile', docker: 'Dockerfile',
  java: 'Java',
};

interface CodeBlockProps {
  language: string;
  code: string;
}

export default function CodeBlock({ language, code }: CodeBlockProps) {
  const [copied, setCopied] = useState(false);
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(() => {
    return () => {
      if (timerRef.current) clearTimeout(timerRef.current);
    };
  }, []);

  const handleCopy = useCallback(() => {
    const fallback = () => {
      const ta = document.createElement('textarea');
      ta.value = code;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
    };
    if (navigator.clipboard?.writeText) {
      navigator.clipboard.writeText(code).catch(fallback);
    } else {
      fallback();
    }
    setCopied(true);
    if (timerRef.current) clearTimeout(timerRef.current);
    timerRef.current = setTimeout(() => setCopied(false), 2000);
  }, [code]);

  const lang = language.toLowerCase();
  const label = LANG_LABELS[lang] || (lang ? lang.charAt(0).toUpperCase() + lang.slice(1) : '');
  const lineCount = code.split('\n').length;

  return (
    <div data-testid="code-block" className="relative group rounded-lg overflow-hidden border border-border-subtle my-3">
      {/* Header bar: copy left, language right */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-elevated/60 border-b border-border-subtle">
        <button
          type="button"
          onClick={handleCopy}
          data-testid="code-copy-button"
          className="flex items-center gap-1 text-[10px] font-mono text-fg-muted hover:text-fg transition-colors cursor-pointer"
        >
          {copied ? (
            <>
              <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="#4ade80" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
                <path d="M3 8l3 3 7-7" />
              </svg>
              Copied!
            </>
          ) : (
            <>
              <svg width="12" height="12" viewBox="0 0 16 16" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round">
                <rect x="5" y="5" width="9" height="9" rx="1.5" />
                <path d="M3 11V3a1.5 1.5 0 0 1 1.5-1.5H11" />
              </svg>
              Copy
            </>
          )}
        </button>
        <span data-testid="code-language-label" className="text-[10px] font-mono uppercase tracking-wider text-fg-muted">
          {label || 'Code'}
        </span>
      </div>

      {/* Highlighted code with max height */}
      <div className="max-h-[500px] overflow-y-auto">
        <SyntaxHighlighter
          language={lang || 'text'}
          style={codeTheme}
          showLineNumbers={lineCount > 5}
          lineNumberStyle={{ color: '#71717a', opacity: 0.4, fontSize: '0.7rem', paddingRight: '1em' }}
          wrapLongLines
        >
          {code}
        </SyntaxHighlighter>
      </div>
    </div>
  );
}
