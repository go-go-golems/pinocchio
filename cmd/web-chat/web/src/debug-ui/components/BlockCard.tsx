import { useState } from 'react';
import type { ParsedBlock } from '../types';
import { safeStringify, truncateText } from '../ui/format/text';
import { getBlockPresentation } from '../ui/presentation/blocks';

export interface BlockCardProps {
  block: ParsedBlock;
  isNew?: boolean;
  showIndex?: boolean;
  compact?: boolean;
  onClick?: () => void;
  expanded?: boolean;
}

export function BlockCard({ block, isNew = false, showIndex = true, compact = false, onClick, expanded: controlledExpanded }: BlockCardProps) {
  const [showRaw, setShowRaw] = useState(false);
  const [internalExpanded, setInternalExpanded] = useState(false);
  const expanded = controlledExpanded ?? internalExpanded;
  const { index, kind, role, payload, metadata } = block;
  
  const middlewareSource = metadata['geppetto.middleware@v1'] as string | undefined;
  const text = payload.text as string | undefined;
  const toolName = payload.name as string | undefined;
  const toolArgs = payload.args as Record<string, unknown> | undefined;
  const toolResult = payload.result as unknown;
  const toolId = payload.id as string | undefined;
  const kindPresentation = getBlockPresentation(kind);

  const hasMetadata = Object.keys(metadata).length > 0;
  
  const handleClick = () => {
    if (onClick) {
      onClick();
    } else {
      setInternalExpanded(!internalExpanded);
    }
  };

  return (
    <div 
      className={`card block-kind-${kind} ${expanded ? 'selected' : ''}`} 
      style={{ padding: compact ? '8px' : '12px', cursor: 'pointer' }}
      onClick={handleClick}
    >
      {/* Header */}
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2">
          {showIndex && (
            <span className="text-xs text-muted" style={{ minWidth: '24px' }}>
              #{index}
            </span>
          )}
          <span className={`badge ${kindPresentation.badgeClass}`}>
            {kindPresentation.icon} {kind}
          </span>
          {role && role !== kind && (
            <span className="text-xs text-secondary">({role})</span>
          )}
          {isNew && <span className="badge badge-green">NEW</span>}
          {hasMetadata && (
            <span className="text-xs text-muted" title="Has metadata">
              📋 {Object.keys(metadata).length}
            </span>
          )}
        </div>
        
        <div className="flex items-center gap-2">
          {middlewareSource && (
            <span className="text-xs text-muted" title="Middleware source">
              via {middlewareSource}
            </span>
          )}
          <button
            className="btn btn-ghost text-xs"
            onClick={(e) => { e.stopPropagation(); setShowRaw(!showRaw); }}
            style={{ padding: '4px 8px' }}
          >
            {showRaw ? '📖 Text' : '{ } JSON'}
          </button>
          <span className="text-xs text-muted">
            {expanded ? '▼' : '▶'}
          </span>
        </div>
      </div>

      {/* Content */}
      {showRaw ? (
        <pre style={{ fontSize: '12px', maxHeight: '200px', overflow: 'auto' }}>
          {safeStringify({ payload, metadata }, 2)}
        </pre>
      ) : (
        <div className="block-content">
          {/* Text content */}
          {text && (
            <div className="text-sm" style={{ whiteSpace: 'pre-wrap' }}>
              {truncateText(text, compact ? 100 : 500)}
            </div>
          )}

          {/* Tool call */}
          {kind === 'tool_call' && (
            <div className="text-sm">
              <div className="flex items-center gap-2 mb-1">
                <span className="badge badge-yellow">{toolName}</span>
                {toolId && (
                  <span className="text-xs text-muted" style={{ fontFamily: 'monospace' }}>
                    {toolId}
                  </span>
                )}
              </div>
              {toolArgs && (
                <pre style={{ fontSize: '11px', marginTop: '4px' }}>
                  {safeStringify(toolArgs, 2)}
                </pre>
              )}
            </div>
          )}

          {/* Tool result */}
          {kind === 'tool_use' && (
            <div className="text-sm">
              {toolId && (
                <span className="text-xs text-muted" style={{ fontFamily: 'monospace' }}>
                  Result for {toolId}
                </span>
              )}
              <pre style={{ fontSize: '11px', marginTop: '4px' }}>
                {safeStringify(toolResult, 2)}
              </pre>
            </div>
          )}
        </div>
      )}

      {/* Expanded metadata section */}
      {expanded && hasMetadata && (
        <div 
          className="metadata-panel mt-3 pt-3" 
          style={{ borderTop: '1px solid var(--border-color)' }}
          onClick={(e) => e.stopPropagation()}
        >
          <h4 className="text-xs text-muted mb-2">Block Metadata</h4>
          <div className="metadata-list">
            {Object.entries(metadata).map(([key, value]) => (
              <MetadataItem key={key} name={key} value={value} />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

interface MetadataItemProps {
  name: string;
  value: unknown;
}

function MetadataItem({ name, value }: MetadataItemProps) {
  const [expanded, setExpanded] = useState(false);
  const isComplex = typeof value === 'object' && value !== null;
  const displayValue = isComplex ? safeStringify(value) : String(value);
  const isLong = displayValue.length > 50;

  return (
    <div 
      className="metadata-item mb-2 p-2" 
      style={{ 
        background: 'var(--bg-secondary)', 
        borderRadius: '4px',
        cursor: isLong || isComplex ? 'pointer' : 'default',
      }}
      onClick={() => (isLong || isComplex) && setExpanded(!expanded)}
    >
      <div className="flex items-center justify-between">
        <span className="text-xs" style={{ color: 'var(--accent-cyan)', fontFamily: 'monospace' }}>
          {name}
        </span>
        {(isLong || isComplex) && (
          <span className="text-xs text-muted">{expanded ? '▼' : '▶'}</span>
        )}
      </div>
      {expanded ? (
        <pre className="text-xs mt-1" style={{ whiteSpace: 'pre-wrap', wordBreak: 'break-all' }}>
          {isComplex ? safeStringify(value, 2) : displayValue}
        </pre>
      ) : (
        <div className="text-xs text-secondary mt-1" style={{ 
          overflow: 'hidden', 
          textOverflow: 'ellipsis', 
          whiteSpace: 'nowrap' 
        }}>
          {isLong ? `${displayValue.slice(0, 50)}...` : displayValue}
        </div>
      )}
    </div>
  );
}

export default BlockCard;
