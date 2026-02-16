import { parse as parseYAML } from 'yaml';
import type { BlockKind, ParsedBlock, ParsedTurn } from '../types';

const BLOCK_KINDS = new Set<BlockKind>([
  'system',
  'user',
  'llm_text',
  'tool_call',
  'tool_use',
  'reasoning',
  'other',
]);

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {};
}

function asString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

export function toBlockKind(raw: unknown): BlockKind {
  if (typeof raw === 'number' && Number.isFinite(raw)) {
    switch (raw) {
      case 0:
        return 'user';
      case 1:
        return 'llm_text';
      case 2:
        return 'tool_call';
      case 3:
        return 'tool_use';
      case 4:
        return 'system';
      case 5:
        return 'reasoning';
      default:
        return 'other';
    }
  }
  const kind = asString(raw);
  const upperKind = kind.toUpperCase();
  if (upperKind === 'SYSTEM' || upperKind === 'BLOCK_KIND_SYSTEM') return 'system';
  if (upperKind === 'USER' || upperKind === 'BLOCK_KIND_USER') return 'user';
  if (upperKind === 'LLM_TEXT' || upperKind === 'BLOCK_KIND_LLM_TEXT') return 'llm_text';
  if (upperKind === 'TOOL_CALL' || upperKind === 'BLOCK_KIND_TOOL_CALL') return 'tool_call';
  if (upperKind === 'TOOL_USE' || upperKind === 'BLOCK_KIND_TOOL_USE') return 'tool_use';
  if (upperKind === 'REASONING' || upperKind === 'BLOCK_KIND_REASONING') return 'reasoning';
  if (BLOCK_KINDS.has(kind as BlockKind)) {
    return kind as BlockKind;
  }
  return 'other';
}

export function toParsedBlock(raw: unknown, index: number): ParsedBlock {
  const obj = asRecord(raw);
  return {
    index,
    id: asString(obj.id ?? obj.ID) || undefined,
    kind: toBlockKind(obj.kind ?? obj.Kind),
    role: asString(obj.role ?? obj.Role) || undefined,
    payload: asRecord(obj.payload ?? obj.Payload),
    metadata: asRecord(obj.metadata ?? obj.Metadata),
  };
}

export function toParsedTurn(raw: unknown, fallbackID = ''): ParsedTurn {
  const obj = asRecord(raw);
  const blocksRaw = Array.isArray(obj.blocks)
    ? obj.blocks
    : Array.isArray(obj.Blocks)
      ? obj.Blocks
      : [];
  return {
    id: asString(obj.id ?? obj.ID) || fallbackID,
    blocks: blocksRaw.map((block, idx) => toParsedBlock(block, idx)),
    metadata: asRecord(obj.metadata ?? obj.Metadata),
    data: asRecord(obj.data ?? obj.Data),
  };
}

export function parseTurnPayload(payload: unknown, fallbackID = ''): ParsedTurn {
  if (typeof payload === 'string') {
    if (!payload.trim()) {
      return { id: fallbackID, blocks: [], metadata: {}, data: {} };
    }
    try {
      return toParsedTurn(parseYAML(payload), fallbackID);
    } catch {
      return { id: fallbackID, blocks: [], metadata: {}, data: {} };
    }
  }
  if (payload && typeof payload === 'object') {
    return toParsedTurn(payload, fallbackID);
  }
  if (!payload) {
    return { id: fallbackID, blocks: [], metadata: {}, data: {} };
  }
  return { id: fallbackID, blocks: [], metadata: {}, data: {} };
}
