import { describe, expect, it } from 'vitest';
import { normalizeAgentModeAnalysis } from './agentModeMarkdown';

describe('normalizeAgentModeAnalysis', () => {
  it('keeps plain prose unchanged', () => {
    expect(normalizeAgentModeAnalysis('No bullets here.')).toBe('No bullets here.');
  });

  it('converts flattened bullet glyph prose into markdown list items', () => {
    const input =
      '• The user asked to switch modes. • Needed capabilities: regex design. • Benefits: faster rule authoring.';
    expect(normalizeAgentModeAnalysis(input)).toBe(
      ['- The user asked to switch modes.', '- Needed capabilities: regex design.', '- Benefits: faster rule authoring.'].join('\n'),
    );
  });

  it('preserves intro prose before bullet glyph items', () => {
    const input = 'Switching is justified. • Needed capabilities: regex design. • Benefits: faster rule authoring.';
    expect(normalizeAgentModeAnalysis(input)).toBe(
      ['Switching is justified.', '', '- Needed capabilities: regex design.', '- Benefits: faster rule authoring.'].join('\n'),
    );
  });

  it('converts line-start bullet glyphs into markdown bullets', () => {
    const input = ['• First point', '• Second point'].join('\n');
    expect(normalizeAgentModeAnalysis(input)).toBe(['- First point', '- Second point'].join('\n'));
  });
});
