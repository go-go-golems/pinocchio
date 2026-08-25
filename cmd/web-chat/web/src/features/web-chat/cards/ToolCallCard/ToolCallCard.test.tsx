import { renderToStaticMarkup } from 'react-dom/server';
import { describe, expect, it } from 'vitest';
import { toolCallEntity } from '../fixtures';
import { ToolCallCard } from './ToolCallCard';

describe('ToolCallCard', () => {
  it('renders human controls only with a complete assigned executor', () => {
    const props = {
      name: 'app.confirm_action',
      status: 'requested',
      sessionId: 'session-1',
      toolCallId: 'call-1',
      input: { title: 'Confirm action', confirmLabel: 'Approve', cancelLabel: 'Deny' },
    };
    const missing = renderToStaticMarkup(<ToolCallCard e={toolCallEntity('missing', props)} />);
    const assigned = renderToStaticMarkup(
      <ToolCallCard
        e={toolCallEntity('assigned', {
          ...props,
          executor: { clientInstanceId: 'client-a', connectionId: 'connection-a', assignmentId: 'assignment-a' },
        })}
      />,
    );

    expect(missing).not.toContain('>Approve<');
    expect(assigned).toContain('>Approve<');
    expect(assigned).toContain('>Deny<');
  });

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
