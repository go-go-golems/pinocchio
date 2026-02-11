export function fmtShort(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return '0';
  if (n < 1000) return String(n);
  if (n < 1_000_000) return `${(n / 1000).toFixed(1)}k`;
  return `${(n / 1_000_000).toFixed(1)}m`;
}

export function fmtSentAt(n: number): string {
  if (!Number.isFinite(n) || n <= 0) return 'sent at —';
  const dt = new Date(n);
  if (!Number.isFinite(dt.getTime())) return 'sent at —';
  return `sent at ${dt.toLocaleTimeString()}`;
}
