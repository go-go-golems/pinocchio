import { WidgetOutlet } from '@go-go-golems/chat-provider';
import { asRecord } from '../provider-support/providerTimeline';
import type { RenderEntity } from '../types';

export function ProviderWidgetRenderer({ e }: { e: RenderEntity }) {
  return (
    <WidgetOutlet
      instanceId={String(e.props?.instanceId ?? e.id)}
      widgetName={String(e.props?.widgetName ?? 'widget')}
      status={String(e.props?.status ?? 'unknown')}
      props={asRecord(e.props?.props)}
    />
  );
}
