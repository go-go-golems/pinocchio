import { ChatProvider } from '@go-go-golems/chat-provider';
import { useMemo } from 'react';
import { basePrefixFromLocation } from '../../utils/basePrefix';
import { ProviderMultiDemoPanel } from './ProviderMultiDemoPanel';

export function ProviderMultiDemoInstance({ name, prompt }: { name: string; prompt: string }) {
  const basePrefix = basePrefixFromLocation();
  const config = useMemo(
    () => ({
      basePrefix,
      sessionIdParam: '',
      sessionStorageKey: `pinocchio.web-chat.multi.${name}.sessionId`,
      createSessionBody: () => ({ profile: 'default' }),
      sendMessageBody: ({ prompt: text }: { prompt: string }) => ({ prompt: text, profile: 'default' }),
    }),
    [basePrefix, name],
  );

  return (
    <ChatProvider config={config}>
      <ProviderMultiDemoPanel name={name} prompt={prompt} />
    </ChatProvider>
  );
}
