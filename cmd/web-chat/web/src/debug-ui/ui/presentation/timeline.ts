export interface TimelineKindPresentation {
  icon: string;
  color: string;
  badgeClass: string;
}

export function getTimelineKindPresentation(kind: string): TimelineKindPresentation {
  switch (kind) {
    case 'message':
      return { icon: '💬', color: 'var(--accent-blue)', badgeClass: 'badge-blue' };
    case 'tool_call':
      return { icon: '🔧', color: 'var(--accent-yellow)', badgeClass: 'badge-yellow' };
    case 'tool_result':
      return { icon: '📤', color: 'var(--accent-cyan)', badgeClass: 'badge-cyan' };
    case 'planning':
      return { icon: '📋', color: 'var(--accent-green)', badgeClass: 'badge-green' };
    case 'log':
      return { icon: '📝', color: 'var(--text-muted)', badgeClass: 'badge-blue' };
    default:
      return { icon: '📦', color: 'var(--border-color)', badgeClass: 'badge-blue' };
  }
}
