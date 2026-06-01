import { ToolCallOutlet } from '@go-go-golems/chat-provider';
import { ToolCallCard } from '../cards';
import type { RenderEntity } from '../types';

export function ProviderToolCallRenderer({ e }: { e: RenderEntity }) {
  const mode = String(e.props?.mode ?? '');
  const isFrontendTool = mode.includes('FRONTEND');

  if (!isFrontendTool) {
    return <ToolCallCard e={e} />;
  }

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
