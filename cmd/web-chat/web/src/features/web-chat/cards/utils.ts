export function asRecord(value: unknown): Record<string, unknown> {
  if (value && typeof value === 'object' && !Array.isArray(value)) {
    return value as Record<string, unknown>;
  }
  return {};
}

export function formatJson(value: unknown): string {
  return JSON.stringify(value ?? {}, null, 2);
}
