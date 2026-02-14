export function truncateText(text: string | undefined | null, maxLength: number): string {
  if (!text) {
    return '';
  }
  if (text.length <= maxLength) {
    return text;
  }
  return `${text.slice(0, maxLength)}...`;
}

export function safeStringify(value: unknown, spacing = 0): string {
  try {
    const serialized = JSON.stringify(value, null, spacing);
    return serialized ?? 'null';
  } catch {
    return '[unserializable]';
  }
}

export function safeString(value: unknown): string {
  if (typeof value === 'string') {
    return value;
  }
  if (value == null) {
    return '';
  }
  return safeStringify(value);
}
