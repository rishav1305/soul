import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { Components } from 'react-markdown';

interface MarkdownProps {
  content: string;
}

const components: Components = {
  code({ className, children, ...rest }) {
    const match = /language-(\w+)/.exec(className || '');
    const isBlock = match || (typeof children === 'string' && children.includes('\n'));

    if (isBlock) {
      return (
        <div className="relative my-2">
          {match && (
            <div className="absolute top-0 right-0 px-2 py-0.5 text-xs text-zinc-500 select-none">
              {match[1]}
            </div>
          )}
          <code
            className={`block bg-zinc-900/50 rounded-lg p-4 text-sm font-mono text-zinc-200 overflow-x-auto ${className || ''}`}
            {...rest}
          >
            {children}
          </code>
        </div>
      );
    }

    return (
      <code
        className="bg-zinc-800 px-1 py-0.5 rounded text-sm font-mono"
        {...rest}
      >
        {children}
      </code>
    );
  },

  pre({ children }) {
    return (
      <pre className="overflow-x-auto my-2">{children}</pre>
    );
  },

  a({ children, href, ...rest }) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        className="text-blue-400 underline hover:text-blue-300"
        {...rest}
      >
        {children}
      </a>
    );
  },

  ul({ children, ...rest }) {
    return (
      <ul className="list-disc list-inside ml-2 my-1 space-y-0.5" {...rest}>
        {children}
      </ul>
    );
  },

  ol({ children, ...rest }) {
    return (
      <ol className="list-decimal list-inside ml-2 my-1 space-y-0.5" {...rest}>
        {children}
      </ol>
    );
  },

  blockquote({ children, ...rest }) {
    return (
      <blockquote
        className="border-l-2 border-zinc-600 pl-3 my-2 text-zinc-400 italic"
        {...rest}
      >
        {children}
      </blockquote>
    );
  },

  table({ children, ...rest }) {
    return (
      <div className="overflow-x-auto my-2">
        <table className="min-w-full border-collapse text-sm" {...rest}>
          {children}
        </table>
      </div>
    );
  },

  thead({ children, ...rest }) {
    return (
      <thead className="border-b border-zinc-700" {...rest}>
        {children}
      </thead>
    );
  },

  th({ children, ...rest }) {
    return (
      <th className="px-3 py-1.5 text-left text-zinc-300 font-medium" {...rest}>
        {children}
      </th>
    );
  },

  td({ children, ...rest }) {
    return (
      <td className="px-3 py-1.5 border-t border-zinc-800" {...rest}>
        {children}
      </td>
    );
  },

  p({ children, ...rest }) {
    return (
      <p className="my-1.5 leading-relaxed" {...rest}>
        {children}
      </p>
    );
  },

  h1({ children, ...rest }) {
    return <h1 className="text-xl font-bold mt-4 mb-2" {...rest}>{children}</h1>;
  },

  h2({ children, ...rest }) {
    return <h2 className="text-lg font-bold mt-3 mb-1.5" {...rest}>{children}</h2>;
  },

  h3({ children, ...rest }) {
    return <h3 className="text-base font-semibold mt-2 mb-1" {...rest}>{children}</h3>;
  },

  hr() {
    return <hr className="border-zinc-700 my-3" />;
  },
};

export function Markdown({ content }: MarkdownProps) {
  if (!content) {
    return null;
  }

  return (
    <div data-testid="markdown-content" className="text-zinc-100 break-words">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {content}
      </ReactMarkdown>
    </div>
  );
}
