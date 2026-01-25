export function toNumber(value: unknown): number | undefined {
  if (typeof value === 'number' && Number.isFinite(value)) return value;
  if (typeof value === 'bigint') return Number(value);
  if (typeof value === 'string') {
    const num = Number(value);
    if (Number.isFinite(num)) return num;
  }
  return undefined;
}

export function toNumberOr(value: unknown, fallback: number): number {
  const num = toNumber(value);
  if (typeof num === 'number') return num;
  return fallback;
}
