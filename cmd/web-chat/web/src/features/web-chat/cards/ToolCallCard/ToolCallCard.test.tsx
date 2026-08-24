import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { toolCallEntity } from '../fixtures';
import { ToolCallCard } from './ToolCallCard';

describe('ToolCallCard', () => {
  it.each(['cancelled', 'timeout'])('hides human controls for terminal %s calls', (status) => {
    const html = renderToStaticMarkup(
      <ToolCallCard
        e={toolCallEntity(`terminal-${status}`, {
          name: 'app.confirm_action',
          status,
          sessionId: 'session-1',
          toolCallId: 'call-1',
          input: { title: 'Confirm action', confirmLabel: 'Approve', cancelLabel: 'Deny' },
        })}
      />,
    );

    expect(html).toContain('app.confirm_action (done)');
    expect(html).not.toContain('>Approve<');
    expect(html).not.toContain('>Deny<');
  });
});
