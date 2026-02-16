import type { BlockKind } from '../../types';

export interface BlockPresentation {
  icon: string;
  badgeClass: string;
}

export function getBlockPresentation(kind: BlockKind | string): BlockPresentation {
  switch (kind) {
    case 'system':
      return { icon: '⚙️', badgeClass: 'badge-purple' };
    case 'user':
      return { icon: '👤', badgeClass: 'badge-blue' };
    case 'llm_text':
      return { icon: '🤖', badgeClass: 'badge-green' };
    case 'tool_call':
      return { icon: '🔧', badgeClass: 'badge-yellow' };
    case 'tool_use':
      return { icon: '📤', badgeClass: 'badge-cyan' };
    case 'reasoning':
      return { icon: '💭', badgeClass: 'badge-red' };
    default:
      return { icon: '📦', badgeClass: 'badge-blue' };
  }
}
