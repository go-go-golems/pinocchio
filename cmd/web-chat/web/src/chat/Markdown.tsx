import React, { useCallback, useMemo, useState } from 'react';
import ReactMarkdown from 'react-markdown';

type MarkdownProps = {
  text: string;
  className?: string;
};

function CopyButton({ getText }: { getText: () => string }) {
  const [copied, setCopied] = useState(false);
  const onCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(getText());
      setCopied(true);
      setTimeout(() => setCopied(false), 900);
    } catch {
      // ignore
    }
  }, [getText]);

  return (
    <button type="button" className="btn btnGhost" onClick={onCopy}>
      {copied ? 'Copied' : 'Copy'}
    </button>
  );
}

export function Markdown({ text, className }: MarkdownProps) {
  const components = useMemo(
    () => ({
      pre({ children }: any) {
        const raw = String(children?.props?.children ?? '');
        return (
          <div>
            <div className="toolbar" style={{ marginBottom: 6 }}>
              <CopyButton getText={() => raw} />
            </div>
            <pre>{children}</pre>
          </div>
        );
      },
      code({ inline, children }: any) {
        if (inline) return <code>{children}</code>;
        return <code>{children}</code>;
      },
      a({ href, children }: any) {
        const safeHref = typeof href === 'string' ? href : '';
        return (
          <a href={safeHref} target="_blank" rel="noreferrer" style={{ color: 'var(--accent)' }}>
            {children}
          </a>
        );
      },
    }),
    [],
  );

  return (
    <div className={className ? `md ${className}` : 'md'}>
      <ReactMarkdown components={components as any}>{text}</ReactMarkdown>
    </div>
  );
}

