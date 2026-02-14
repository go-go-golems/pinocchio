export interface EventPresentation {
  icon: string;
  color: string;
  badgeClass: string;
}

export interface EventPresentationOptions {
  thinkingIcon?: 'thought' | 'bot';
  logColor?: string;
  unknownColor?: string;
  unknownBadgeClass?: string;
}

export function getEventPresentation(
  type: string,
  options: EventPresentationOptions = {},
): EventPresentation {
  const thinkingIcon = options.thinkingIcon === 'bot' ? '🤖' : '💭';
  const logColor = options.logColor ?? 'var(--text-muted)';
  const unknownColor = options.unknownColor ?? 'var(--border-color)';
  const unknownBadgeClass = options.unknownBadgeClass ?? 'badge-blue';

  if (type.startsWith('llm.')) {
    if (type === 'llm.start') {
      return { icon: '▶️', color: 'var(--accent-green)', badgeClass: 'badge-green' };
    }
    if (type === 'llm.delta') {
      return { icon: '📝', color: 'var(--accent-blue)', badgeClass: 'badge-blue' };
    }
    if (type === 'llm.final') {
      return { icon: '✅', color: 'var(--accent-green)', badgeClass: 'badge-green' };
    }
    if (type.includes('thinking')) {
      return { icon: thinkingIcon, color: 'var(--accent-purple)', badgeClass: 'badge-purple' };
    }
    return { icon: '🤖', color: 'var(--accent-blue)', badgeClass: 'badge-blue' };
  }

  if (type.startsWith('tool.')) {
    if (type === 'tool.start') {
      return { icon: '🔧', color: 'var(--accent-yellow)', badgeClass: 'badge-yellow' };
    }
    if (type === 'tool.result') {
      return { icon: '📤', color: 'var(--accent-cyan)', badgeClass: 'badge-cyan' };
    }
    if (type === 'tool.done') {
      return { icon: '✓', color: 'var(--accent-green)', badgeClass: 'badge-green' };
    }
    return { icon: '🔧', color: 'var(--accent-yellow)', badgeClass: 'badge-yellow' };
  }

  if (type === 'log') {
    return { icon: '📋', color: logColor, badgeClass: 'badge-blue' };
  }

  return { icon: '📦', color: unknownColor, badgeClass: unknownBadgeClass };
}
