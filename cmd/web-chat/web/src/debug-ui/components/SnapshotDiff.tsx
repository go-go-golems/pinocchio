import React from 'react';
import type { ParsedBlock, ParsedTurn } from '../types';
import { formatPhaseLabel } from '../ui/format/phase';
import { truncateText } from '../ui/format/text';

export type BlockDiffStatus = 'same' | 'added' | 'removed' | 'changed' | 'reordered';

export interface BlockDiff {
  status: BlockDiffStatus;
  blockA?: ParsedBlock;
  blockB?: ParsedBlock;
  changes?: string[]; // What changed: 'payload', 'metadata', 'position'
}

export interface SnapshotDiffProps {
  phaseA: string;
  phaseB: string;
  turnA: ParsedTurn;
  turnB: ParsedTurn;
  onBlockSelect?: (block: ParsedBlock, side: 'A' | 'B') => void;
}

export function SnapshotDiff({ phaseA, phaseB, turnA, turnB, onBlockSelect }: SnapshotDiffProps) {
  const diffs = computeBlockDiffs(turnA.blocks, turnB.blocks);
  const summary = computeDiffSummary(diffs);
  const metaDiff = computeMetadataDiff(turnA.metadata, turnB.metadata);

  return (
    <div className="snapshot-diff">
      {/* Header */}
      <DiffHeader phaseA={phaseA} phaseB={phaseB} />

      {/* Summary bar */}
      <DiffSummaryBar summary={summary} />

      {/* Side by side blocks */}
      <div className="diff-body">
        <div className="diff-column">
          <div className="diff-column-header">{phaseA}</div>
        </div>
        <div className="diff-column">
          <div className="diff-column-header">{phaseB}</div>
        </div>
      </div>

      <div className="diff-rows">
        {diffs.map((diff, idx) => (
          <DiffBlockRow
            key={idx}
            diff={diff}
            onSelectA={() => diff.blockA && onBlockSelect?.(diff.blockA, 'A')}
            onSelectB={() => diff.blockB && onBlockSelect?.(diff.blockB, 'B')}
          />
        ))}
      </div>

      {/* Metadata diff */}
      {metaDiff.length > 0 && (
        <div className="metadata-diff-section">
          <h4>Metadata Changes</h4>
          <MetadataDiff changes={metaDiff} />
        </div>
      )}
    </div>
  );
}

interface DiffHeaderProps {
  phaseA: string;
  phaseB: string;
}

function DiffHeader({ phaseA, phaseB }: DiffHeaderProps) {
  return (
    <div className="diff-header">
      <span className="phase-label phase-a">{formatPhaseLabel(phaseA)}</span>
      <span className="diff-arrow">→</span>
      <span className="phase-label phase-b">{formatPhaseLabel(phaseB)}</span>
    </div>
  );
}

interface DiffSummary {
  added: number;
  removed: number;
  changed: number;
  reordered: number;
  same: number;
}

interface DiffSummaryBarProps {
  summary: DiffSummary;
}

function DiffSummaryBar({ summary }: DiffSummaryBarProps) {
  return (
    <div className="diff-summary-bar">
      {summary.added > 0 && (
        <span className="summary-chip added">+{summary.added} added</span>
      )}
      {summary.removed > 0 && (
        <span className="summary-chip removed">-{summary.removed} removed</span>
      )}
      {summary.changed > 0 && (
        <span className="summary-chip changed">~{summary.changed} changed</span>
      )}
      {summary.reordered > 0 && (
        <span className="summary-chip reordered">↔{summary.reordered} reordered</span>
      )}
      {summary.same > 0 && (
        <span className="summary-chip same">{summary.same} unchanged</span>
      )}
    </div>
  );
}

interface DiffBlockRowProps {
  diff: BlockDiff;
  onSelectA?: () => void;
  onSelectB?: () => void;
}

function DiffBlockRow({ diff, onSelectA, onSelectB }: DiffBlockRowProps) {
  const { status, blockA, blockB, changes } = diff;

  return (
    <div className={`diff-block-row status-${status}`}>
      <div className="diff-cell left" onClick={onSelectA}>
        {blockA ? (
          <BlockPreview block={blockA} dimmed={status === 'removed'} />
        ) : (
          <div className="empty-cell" />
        )}
      </div>

      <div className="diff-status">
        <span className={`status-badge status-${status}`}>
          {getStatusIcon(status)}
        </span>
        {changes && changes.length > 0 && (
          <div className="change-list">
            {changes.map((c, i) => (
              <span key={i} className="change-chip">{c}</span>
            ))}
          </div>
        )}
      </div>

      <div className="diff-cell right" onClick={onSelectB}>
        {blockB ? (
          <BlockPreview block={blockB} dimmed={status === 'added'} highlighted={status === 'added' || status === 'changed'} />
        ) : (
          <div className="empty-cell" />
        )}
      </div>
    </div>
  );
}

interface BlockPreviewProps {
  block: ParsedBlock;
  dimmed?: boolean;
  highlighted?: boolean;
}

function BlockPreview({ block, dimmed, highlighted }: BlockPreviewProps) {
  const { index, kind, payload } = block;
  const text = payload.text as string | undefined;
  const name = payload.name as string | undefined;

  return (
    <div 
      className={`block-preview block-kind-${kind} ${dimmed ? 'dimmed' : ''} ${highlighted ? 'highlighted' : ''}`}
    >
      <div className="block-preview-header">
        <span className="block-index">#{index}</span>
        <span className="block-kind">{kind}</span>
      </div>
      <div className="block-preview-content">
        {text && truncateText(text, 50)}
        {name && <span className="tool-name">{name}</span>}
        {!text && !name && <span className="no-content">—</span>}
      </div>
    </div>
  );
}

interface MetadataChange {
  key: string;
  type: 'added' | 'removed' | 'changed';
  valueA?: unknown;
  valueB?: unknown;
}

interface MetadataDiffProps {
  changes: MetadataChange[];
}

function MetadataDiff({ changes }: MetadataDiffProps) {
  return (
    <div className="metadata-diff">
      {changes.map((change, idx) => (
        <div key={idx} className={`meta-change meta-${change.type}`}>
          <span className="meta-key">{change.key}</span>
          <span className="meta-type">{change.type}</span>
        </div>
      ))}
    </div>
  );
}

function getStatusIcon(status: BlockDiffStatus): string {
  switch (status) {
    case 'same': return '=';
    case 'added': return '+';
    case 'removed': return '−';
    case 'changed': return '~';
    case 'reordered': return '↔';
  }
}

// Diff computation - identity-aware (uses block id or kind+index as fallback)
function computeBlockDiffs(blocksA: ParsedBlock[], blocksB: ParsedBlock[]): BlockDiff[] {
  const diffs: BlockDiff[] = [];
  const matchedB = new Set<number>();

  // First pass: match by id if available
  for (const blockA of blocksA) {
    const id = blockA.id;
    if (id) {
      const matchIdx = blocksB.findIndex((b, i) => !matchedB.has(i) && b.id === id);
      if (matchIdx >= 0) {
        matchedB.add(matchIdx);
        const blockB = blocksB[matchIdx];
        const status = getBlockStatus(blockA, blockB, matchIdx);
        diffs.push({ 
          status, 
          blockA, 
          blockB,
          changes: status === 'changed' ? getBlockChanges(blockA, blockB) : undefined,
        });
        continue;
      }
    }
    
    // Try matching by kind and content hash
    const matchIdx = blocksB.findIndex((b, i) => 
      !matchedB.has(i) && 
      b.kind === blockA.kind && 
      JSON.stringify(b.payload) === JSON.stringify(blockA.payload)
    );
    
    if (matchIdx >= 0) {
      matchedB.add(matchIdx);
      const blockB = blocksB[matchIdx];
      const status = blockA.index !== blockB.index ? 'reordered' : 'same';
      diffs.push({ status, blockA, blockB });
    } else {
      diffs.push({ status: 'removed', blockA });
    }
  }

  // Add unmatched blocks from B as added
  blocksB.forEach((blockB, i) => {
    if (!matchedB.has(i)) {
      diffs.push({ status: 'added', blockB });
    }
  });

  // Sort by original index
  diffs.sort((a, b) => {
    const idxA = a.blockA?.index ?? a.blockB?.index ?? 999;
    const idxB = b.blockA?.index ?? b.blockB?.index ?? 999;
    return idxA - idxB;
  });

  return diffs;
}

function getBlockStatus(blockA: ParsedBlock, blockB: ParsedBlock, newIndex: number): BlockDiffStatus {
  const payloadSame = JSON.stringify(blockA.payload) === JSON.stringify(blockB.payload);
  const metaSame = JSON.stringify(blockA.metadata) === JSON.stringify(blockB.metadata);
  
  if (payloadSame && metaSame) {
    return blockA.index !== newIndex ? 'reordered' : 'same';
  }
  return 'changed';
}

function getBlockChanges(blockA: ParsedBlock, blockB: ParsedBlock): string[] {
  const changes: string[] = [];
  if (JSON.stringify(blockA.payload) !== JSON.stringify(blockB.payload)) {
    changes.push('payload');
  }
  if (JSON.stringify(blockA.metadata) !== JSON.stringify(blockB.metadata)) {
    changes.push('metadata');
  }
  if (blockA.index !== blockB.index) {
    changes.push('position');
  }
  return changes;
}

function computeDiffSummary(diffs: BlockDiff[]): DiffSummary {
  return diffs.reduce(
    (acc, d) => {
      acc[d.status]++;
      return acc;
    },
    { added: 0, removed: 0, changed: 0, reordered: 0, same: 0 }
  );
}

function computeMetadataDiff(metaA: Record<string, unknown>, metaB: Record<string, unknown>): MetadataChange[] {
  const changes: MetadataChange[] = [];
  const allKeys = new Set([...Object.keys(metaA), ...Object.keys(metaB)]);

  for (const key of allKeys) {
    const inA = key in metaA;
    const inB = key in metaB;

    if (inA && !inB) {
      changes.push({ key, type: 'removed', valueA: metaA[key] });
    } else if (!inA && inB) {
      changes.push({ key, type: 'added', valueB: metaB[key] });
    } else if (JSON.stringify(metaA[key]) !== JSON.stringify(metaB[key])) {
      changes.push({ key, type: 'changed', valueA: metaA[key], valueB: metaB[key] });
    }
  }

  return changes;
}

export default SnapshotDiff;
