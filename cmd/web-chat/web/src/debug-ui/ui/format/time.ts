function toSafeDate(value: string | number | Date): Date | null {
  const date = value instanceof Date ? value : new Date(value);
  return Number.isNaN(date.getTime()) ? null : date;
}

export function formatTimeShort(value: string | number | Date): string {
  const date = toSafeDate(value);
  if (!date) {
    return '—';
  }
  return date.toLocaleTimeString();
}

export function formatDateTime(value: string | number | Date): string {
  const date = toSafeDate(value);
  if (!date) {
    return '—';
  }
  return date.toLocaleString();
}
