import type { ChatDebugEvent } from '@go-go-golems/chat-provider';
import { recordStreamDebug } from '../../../ws/streamDebug';

export function recordProviderDebugEvent(event: ChatDebugEvent) {
  recordStreamDebug(event);
}
