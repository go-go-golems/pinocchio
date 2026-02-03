import type React from 'react';
import type { ChatPart, PartProps } from './types';

export function mergeClassName(...names: Array<string | undefined>): string | undefined {
  const filtered = names.filter(Boolean) as string[];
  if (!filtered.length) return undefined;
  return filtered.join(' ');
}

export function mergeStyle(...styles: Array<React.CSSProperties | undefined>): React.CSSProperties | undefined {
  const merged = Object.assign({}, ...styles.filter(Boolean));
  if (!Object.keys(merged).length) return undefined;
  return merged;
}

export function getPartProps(part: ChatPart, partProps?: PartProps): React.HTMLAttributes<HTMLElement> {
  const raw = partProps?.[part] ?? {};
  const { className, style, ...rest } = raw;
  const { "data-part": _dataPart, ...safeRest } = rest as Record<string, unknown>;
  return {
    ...safeRest,
    className,
    style,
  } as React.HTMLAttributes<HTMLElement>;
}
