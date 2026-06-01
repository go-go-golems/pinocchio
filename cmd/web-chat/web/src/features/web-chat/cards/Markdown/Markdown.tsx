import { useCallback, useMemo, useState } from 'react';
import ReactMarkdown from 'react-markdown';
import remarkGfm from 'remark-gfm';
import { logWarn } from '../../../../utils/logger';
import type { MarkdownProps } from './types';

function isSafeHref(href: string): boolean {
  if (!href) return false;
  if (href.startsWith('#') || href.startsWith('/')) return true;
  try {
    const url = new URL(href);
    return url.protocol === 'http:' || url.protocol === 'https:' || url.protocol === 'mailto:';
  } catch {
    return false;
  }
}

function CopyButton({ getText }: { getText: () => string }) {
  const [copied, setCopied] = useState(false);
  const onCopy = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(getText());
      setCopied(true);
      setTimeout(() => setCopied(false), 900);
    } catch (err) {
      logWarn('clipboard copy failed', { scope: 'markdown.copy' }, err);
    }
  }, [getText]);

  return (
    <button type="button" data-part="button" data-variant="ghost" onClick={onCopy}>
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
            <div data-part="toolbar" data-spacing="bottom">
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
        const safeHref = typeof href === 'string' && isSafeHref(href) ? href : undefined;
        if (!safeHref) {
          return <span data-part="unsafe-link">{children}</span>;
        }
        return (
          <a href={safeHref} target="_blank" rel="noreferrer">
            {children}
          </a>
        );
      },
    }),
    [],
  );

  return (
    <div data-part="markdown" className={className}>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components as any}>
        {text}
      </ReactMarkdown>
    </div>
  );
}
