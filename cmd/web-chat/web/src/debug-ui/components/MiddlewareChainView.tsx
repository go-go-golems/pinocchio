
import type { EngineStats, MwTrace, MwTraceLayer } from '../types';
import { CorrelationIdBar } from './CorrelationIdBar';

export interface MiddlewareChainViewProps {
  trace: MwTrace;
  onLayerClick?: (layer: MwTraceLayer) => void;
}

export function MiddlewareChainView({ trace, onLayerClick }: MiddlewareChainViewProps) {
  const { conv_id, inference_id, chain, engine } = trace;
  const totalDuration = chain.reduce((sum, l) => sum + l.duration_ms, 0) + engine.latency_ms;

  return (
    <div className="middleware-chain-view">
      {/* Header */}
      <CorrelationIdBar convId={conv_id} inferenceId={inference_id} />

      <div className="flex items-center justify-between mb-4">
        <h3>Middleware Chain</h3>
        <div className="text-sm text-muted">
          {chain.length} layers · {totalDuration}ms total
        </div>
      </div>

      {/* Onion visualization */}
      <div className="onion-layers">
        {chain.map((layer, idx) => (
          <MiddlewareLayerCard
            key={idx}
            layer={layer}
            isFirst={idx === 0}
            isLast={idx === chain.length - 1}
            onClick={() => onLayerClick?.(layer)}
          />
        ))}

        {/* Engine center */}
        <EngineCard stats={engine} />

        {/* Return path (reversed order) */}
        {[...chain].reverse().map((layer, idx) => (
          <MiddlewareLayerCard
            key={`return-${idx}`}
            layer={layer}
            isReturn
            onClick={() => onLayerClick?.(layer)}
          />
        ))}
      </div>
    </div>
  );
}

interface MiddlewareLayerCardProps {
  layer: MwTraceLayer;
  isFirst?: boolean;
  isLast?: boolean;
  isReturn?: boolean;
  onClick?: () => void;
}

function MiddlewareLayerCard({ layer, isFirst, isLast, isReturn, onClick }: MiddlewareLayerCardProps) {
  const { name, pre_blocks, post_blocks, blocks_added, blocks_removed, blocks_changed, duration_ms } = layer;
  const hasChanges = blocks_added > 0 || blocks_removed > 0 || blocks_changed > 0;

  return (
    <div
      className="card"
      onClick={onClick}
      style={{
        cursor: onClick ? 'pointer' : 'default',
        marginLeft: isReturn ? 0 : `${layer.layer * 20}px`,
        marginRight: isReturn ? `${layer.layer * 20}px` : 0,
        borderLeft: hasChanges ? '3px solid var(--accent-yellow)' : '3px solid var(--border-color)',
        opacity: isReturn ? 0.7 : 1,
      }}
    >
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-2">
          <span className="text-xs text-muted">
            {isReturn ? '←' : '→'} L{layer.layer}
          </span>
          <span className="badge badge-purple">{name}</span>
        </div>
        <span className="text-xs text-muted">{duration_ms}ms</span>
      </div>

      {!isReturn && (
        <div className="flex items-center gap-3 mt-2 text-xs">
          <span className="text-secondary">
            {pre_blocks} → {post_blocks} blocks
          </span>
          {blocks_added > 0 && <span className="badge badge-green">+{blocks_added}</span>}
          {blocks_removed > 0 && <span className="badge badge-red">-{blocks_removed}</span>}
          {blocks_changed > 0 && <span className="badge badge-yellow">~{blocks_changed}</span>}
        </div>
      )}
    </div>
  );
}

interface EngineCardProps {
  stats: EngineStats;
}

function EngineCard({ stats }: EngineCardProps) {
  const { model, input_blocks, output_blocks, latency_ms, tokens_in, tokens_out, stop_reason } = stats;

  return (
    <div
      className="card"
      style={{
        background: 'var(--bg-tertiary)',
        border: '2px solid var(--accent-blue)',
        margin: '16px 0',
        textAlign: 'center',
      }}
    >
      <div className="mb-2">
        <span className="badge badge-blue">🤖 {model}</span>
      </div>
      
      <div className="text-sm mb-2">
        {input_blocks} blocks → Engine → {output_blocks} blocks
      </div>

      <div className="flex items-center justify-center gap-4 text-xs text-secondary">
        <span>⏱️ {latency_ms}ms</span>
        <span>📥 {tokens_in} tokens</span>
        <span>📤 {tokens_out} tokens</span>
        <span>🏁 {stop_reason}</span>
      </div>
    </div>
  );
}

export default MiddlewareChainView;
