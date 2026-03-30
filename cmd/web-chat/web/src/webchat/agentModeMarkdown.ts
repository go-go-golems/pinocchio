export function normalizeAgentModeAnalysis(text: string): string {
  const normalized = text.replace(/\r\n?/g, '\n').trim();
  if (!normalized.includes('•')) {
    return normalized;
  }

  const lineNormalized = normalized.replace(/^\s*•\s+/gm, '- ');
  if (!lineNormalized.includes(' • ')) {
    return lineNormalized;
  }

  const parts = lineNormalized
    .split(/\s+•\s+/)
    .map((part) => part.trim())
    .filter(Boolean);

  if (parts.length < 2) {
    return lineNormalized;
  }

  if (lineNormalized.trimStart().startsWith('- ')) {
    return parts
      .map((part) => (part.startsWith('- ') ? part : `- ${part}`))
      .join('\n');
  }

  const [intro, ...bullets] = parts;
  if (bullets.length === 0) {
    return lineNormalized;
  }

  return [intro, '', ...bullets.map((part) => `- ${part}`)].join('\n');
}
