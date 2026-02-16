function normalizeIndex(index: number): number {
  if (!Number.isFinite(index)) {
    return 0;
  }

  return Math.max(0, Math.trunc(index));
}

export function shouldApplyDeterministicOverrides(absoluteIndex: number, fixtureCount: number): boolean {
  return fixtureCount > 0 && normalizeIndex(absoluteIndex) >= fixtureCount;
}

export function makeDeterministicId(prefix: string, index: number, padWidth = 3): string {
  const ordinal = normalizeIndex(index) + 1;
  return `${prefix}_${String(ordinal).padStart(padWidth, '0')}`;
}

export function makeDeterministicTimeMs(baseMs: number, index: number, stepMs: number): number {
  return baseMs + normalizeIndex(index) * stepMs;
}

export function makeDeterministicIsoTime(baseMs: number, index: number, stepMs: number): string {
  return new Date(makeDeterministicTimeMs(baseMs, index, stepMs)).toISOString();
}

export function makeDeterministicSeq(baseSeq: number, index: number, step: number): number {
  return baseSeq + normalizeIndex(index) * step;
}
