import { ToolCallOutlet } from '@go-go-golems/chat-provider';
import type { RenderEntity } from '../../../webchat/types';

export function ProviderToolCallRenderer({ e }: { e: RenderEntity }) {
  return (
    <ToolCallOutlet
      toolCallId={String(e.props?.toolCallId ?? e.id)}
      toolName={String(e.props?.toolName ?? e.props?.name ?? 'tool')}
      status={String(e.props?.status ?? 'requested')}
      input={e.props?.input}
      result={e.props?.result}
      error={typeof e.props?.error === 'string' ? e.props.error : undefined}
    />
  );
}
