
import type { ConversationSummary } from '../types';

export interface ConversationCardProps {
  conversation: ConversationSummary;
  selected?: boolean;
  onClick?: () => void;
}

export function ConversationCard({
  conversation,
  selected = false,
  onClick,
}: ConversationCardProps) {
  const { id, profile_slug, session_id, is_running, ws_connections, turn_count, last_activity } =
    conversation;

  const timeAgo = formatTimeAgo(last_activity);

  return (
    <div
      className={`card ${selected ? 'selected' : ''} ${is_running ? 'running' : ''}`}
      onClick={onClick}
      style={{ cursor: onClick ? 'pointer' : 'default' }}
    >
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          <span className={`status-dot ${is_running ? 'running' : 'idle'}`} />
          <span className="text-sm" style={{ fontFamily: 'monospace' }}>
            {id}
          </span>
        </div>
        <span className={`badge ${getProfileBadgeClass(profile_slug)}`}>
          {profile_slug}
        </span>
      </div>

      <div className="flex items-center gap-3 text-sm text-secondary">
        <span title="Session ID">🔗 {session_id.slice(0, 8)}</span>
        <span title="Turn count">💬 {turn_count}</span>
        <span title="WebSocket connections">📡 {ws_connections}</span>
      </div>

      <div className="text-xs text-muted mt-2">{timeAgo}</div>
    </div>
  );
}

function getProfileBadgeClass(profile: string): string {
  switch (profile) {
    case 'agent':
      return 'badge-purple';
    case 'general':
      return 'badge-blue';
    default:
      return 'badge-cyan';
  }
}

function formatTimeAgo(isoString: string): string {
  const date = new Date(isoString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);

  if (diffMins < 1) return 'Just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  
  const diffHours = Math.floor(diffMins / 60);
  if (diffHours < 24) return `${diffHours}h ago`;
  
  const diffDays = Math.floor(diffHours / 24);
  return `${diffDays}d ago`;
}

export default ConversationCard;
