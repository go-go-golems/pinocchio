import { describe, expect, it } from 'vitest';
import { parseTurnPayload, toBlockKind, toParsedTurn } from './turnParsing';

describe('turnParsing', () => {
  it('parses canonical lowercase payload shape', () => {
    const parsed = parseTurnPayload(
      {
        id: 'turn-1',
        blocks: [
          {
            id: 'blk-1',
            kind: 'user',
            role: 'user',
            payload: { text: 'hello' },
          },
        ],
      },
      'fallback'
    );

    expect(parsed.id).toBe('turn-1');
    expect(parsed.blocks).toHaveLength(1);
    expect(parsed.blocks[0]).toMatchObject({
      id: 'blk-1',
      kind: 'user',
      role: 'user',
    });
  });

  it('parses protobuf-style capitalized parsed shape and numeric enums', () => {
    const parsed = toParsedTurn(
      {
        ID: 'turn-2',
        Blocks: [
          {
            ID: 'blk-2',
            Kind: 4,
            Role: 'system',
            Payload: { text: 'You are an assistant' },
          },
          {
            ID: 'blk-3',
            Kind: 1,
            Role: 'assistant',
            Payload: { text: 'Hello' },
          },
        ],
      },
      'fallback'
    );

    expect(parsed.id).toBe('turn-2');
    expect(parsed.blocks).toHaveLength(2);
    expect(parsed.blocks[0].kind).toBe('system');
    expect(parsed.blocks[1].kind).toBe('llm_text');
    expect(parsed.blocks[1].payload).toEqual({ text: 'Hello' });
  });

  it('parses YAML payload strings and preserves block count', () => {
    const yamlPayload = `
id: turn-yaml
blocks:
  - id: blk-y
    kind: user
    role: user
    payload:
      text: hello from yaml
`;

    const parsed = parseTurnPayload(yamlPayload, 'fallback');
    expect(parsed.id).toBe('turn-yaml');
    expect(parsed.blocks).toHaveLength(1);
    expect(parsed.blocks[0].payload).toEqual({ text: 'hello from yaml' });
  });

  it('returns empty fallback turn for invalid payloads', () => {
    const parsed = parseTurnPayload('::::invalid yaml::::', 'fallback-turn');
    expect(parsed.id).toBe('fallback-turn');
    expect(parsed.blocks).toHaveLength(0);
  });

  it('maps block kinds from enum names and numeric values', () => {
    expect(toBlockKind(0)).toBe('user');
    expect(toBlockKind(4)).toBe('system');
    expect(toBlockKind('BLOCK_KIND_TOOL_CALL')).toBe('tool_call');
    expect(toBlockKind('LLM_TEXT')).toBe('llm_text');
    expect(toBlockKind('mystery')).toBe('other');
  });
});
