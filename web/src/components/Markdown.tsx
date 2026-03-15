import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import type { Components } from 'react-markdown';
import CodeBlock from './CodeBlock';

interface MarkdownProps {
  content: string;
}

const components: Components = {
  code({ className, children, node, ...props }) {
    const match = /language-(\w+)/.exec(className || '');
    const isInline = !match && !String(children).includes('\n');
    if (isInline) {
      return <code className="px-1.5 py-0.5 bg-elevated rounded text-[0.85em] font-mono text-soul/90" {...props}>{children}</code>;
    }
    return <CodeBlock language={match?.[1] ?? ''} code={String(children).replace(/\n$/, '')} />;
  },

  pre({ children }) {
    // CodeBlock handles its own wrapping; just pass children through
    return <>{children}</>;
  },

  a({ children, href, node, ...rest }) {
    return (
      <a
        href={href}
        target="_blank"
        rel="noopener noreferrer"
        className="text-soul underline hover:text-soul/80"
        {...rest}
      >
        {children}
      </a>
    );
  },

  ul({ children, node, ...rest }) {
    return (
      <ul className="list-disc ml-5 my-2 space-y-1 text-[0.935rem] leading-[1.7]" {...rest}>
        {children}
      </ul>
    );
  },

  ol({ children, node, ...rest }) {
    return (
      <ol className="list-decimal ml-5 my-2 space-y-1 text-[0.935rem] leading-[1.7]" {...rest}>
        {children}
      </ol>
    );
  },

  li({ children, node, ...rest }) {
    return (
      <li className="pl-1" {...rest}>
        {children}
      </li>
    );
  },

  blockquote({ children, node, ...rest }) {
    return (
      <blockquote
        className="border-l-2 border-border-default pl-3 my-2 text-fg-secondary italic"
        {...rest}
      >
        {children}
      </blockquote>
    );
  },

  table({ children, node, ...rest }) {
    return (
      <div className="overflow-x-auto my-2">
        <table className="min-w-full border-collapse text-sm" {...rest}>
          {children}
        </table>
      </div>
    );
  },

  thead({ children, node, ...rest }) {
    return (
      <thead className="border-b border-border-default" {...rest}>
        {children}
      </thead>
    );
  },

  th({ children, node, ...rest }) {
    return (
      <th className="px-3 py-1.5 text-left text-fg-secondary font-medium" {...rest}>
        {children}
      </th>
    );
  },

  td({ children, node, ...rest }) {
    return (
      <td className="px-3 py-1.5 border-t border-border-subtle" {...rest}>
        {children}
      </td>
    );
  },

  p({ children, node, ...rest }) {
    return (
      <p className="my-2 leading-[1.7] text-[0.935rem]" {...rest}>
        {children}
      </p>
    );
  },

  h1({ children, node, ...rest }) {
    return <h1 className="text-xl font-bold mt-6 mb-2 text-fg tracking-tight" {...rest}>{children}</h1>;
  },

  h2({ children, node, ...rest }) {
    return <h2 className="text-lg font-bold mt-5 mb-2 text-fg tracking-tight" {...rest}>{children}</h2>;
  },

  h3({ children, node, ...rest }) {
    return <h3 className="text-base font-semibold mt-4 mb-1.5 text-fg" {...rest}>{children}</h3>;
  },

  strong({ children, node, ...rest }) {
    return <strong className="font-semibold text-fg" {...rest}>{children}</strong>;
  },

  em({ children, node, ...rest }) {
    return <em className="italic text-fg-secondary" {...rest}>{children}</em>;
  },

  hr() {
    return <hr className="border-border-default my-4" />;
  },

  img({ src, alt, node, ...rest }) {
    return (
      <img
        src={src}
        alt={alt}
        className="rounded-lg border border-border-subtle my-3 max-w-full"
        loading="lazy"
        {...rest}
      />
    );
  },
};

export function Markdown({ content }: MarkdownProps) {
  if (!content) {
    return null;
  }

  return (
    <div data-testid="markdown-content" className="text-fg break-words">
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {content}
      </ReactMarkdown>
    </div>
  );
}
