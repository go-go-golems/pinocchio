import { fmtSentAt } from '../../../../webchat/utils';
import { Markdown } from '../Markdown';
import type { MessageCardProps } from './types';

export function MessageCard({ e }: MessageCardProps) {
  const role = String(e.props?.role ?? 'assistant');
  const content = String(e.props?.content ?? '');
  const error = String(e.props?.error ?? '');
  const streaming = !!e.props?.streaming;
  const roleAttr = role === 'user' || role === 'assistant' || role === 'thinking' ? role : 'assistant';

  return (
    <div data-part="card">
      <div data-part="card-header">
        <div data-part="message-role" data-role={roleAttr}>
          {role}
        </div>
        {streaming ? <div data-part="streaming-dot" /> : null}
        <div data-part="card-header-meta">{fmtSentAt(e.createdAt)}</div>
      </div>
      <div data-part="card-body">
        {error ? (
          <div data-part="error-item">
            <div data-part="pill" data-variant="danger">
              stopped
            </div>
            <div data-part="error-item-detail" data-mono="true">
              {error}
            </div>
          </div>
        ) : null}
        {content ? <Markdown text={content} /> : error ? null : <div data-part="pill">...</div>}
      </div>
    </div>
  );
}
