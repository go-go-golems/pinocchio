import { ToolCallOutlet } from '@go-go-golems/chat-provider';
import { ToolCallCard } from '../cards';
import type { RenderEntity } from '../types';

export function isFrontendToolMode(value: unknown): boolean {
  const mode = String(value ?? '');
  return mode.includes('FRONTEND') || mode === '1' || mode === '2';
}

export function ProviderToolCallRenderer({ e }: { e: RenderEntity }) {
  const isFrontendTool = isFrontendToolMode(e.props?.mode);

  if (!isFrontendTool) {
    return <ToolCallCard e={e} />;
  }

  // ToolCallOutlet is the only actionable frontend-tool surface. It delegates
  // ownership and human completion to chat-provider's ToolRuntime; generic
  // timeline cards remain read-only projections.
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
