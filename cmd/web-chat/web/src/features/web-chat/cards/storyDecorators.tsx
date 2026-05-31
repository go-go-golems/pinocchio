import type { ReactNode } from 'react';

export function CardStoryFrame({ children }: { children: ReactNode }) {
  return (
    <div data-pwchat="" data-part="root" data-theme="default" style={{ minHeight: 320, padding: 18 }}>
      <div data-part="timeline" style={{ maxWidth: 760 }}>
        <div data-part="turn" data-role="assistant">
          <div data-part="bubble">
            <div data-part="content">{children}</div>
          </div>
        </div>
      </div>
    </div>
  );
}
