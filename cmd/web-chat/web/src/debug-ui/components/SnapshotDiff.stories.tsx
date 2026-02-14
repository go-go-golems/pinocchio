import type { Meta, StoryObj } from '@storybook/react';
import type { ParsedBlock, ParsedTurn } from '../types';
import { SnapshotDiff } from './SnapshotDiff';

const meta: Meta<typeof SnapshotDiff> = {
  title: 'Debug UI/SnapshotDiff',
  component: SnapshotDiff,
  parameters: {
    layout: 'padded',
  },
};

export default meta;
type Story = StoryObj<typeof SnapshotDiff>;

// Mock turns for different phases
const preInferenceTurn: ParsedTurn = {
  id: 'turn_01',
  blocks: [
    {
      index: 0,
      kind: 'system',
      role: 'system',
      payload: { text: 'You are a helpful assistant.' },
      metadata: { 'geppetto.middleware@v1': 'system-prompt-mw' },
    },
    {
      index: 1,
      kind: 'user',
      role: 'user',
      payload: { text: 'What is the weather in Paris?' },
      metadata: {},
    },
  ],
  metadata: { 'geppetto.session_id@v1': 'sess_01' },
  data: {},
};

const postInferenceTurn: ParsedTurn = {
  id: 'turn_01',
  blocks: [
    {
      index: 0,
      kind: 'system',
      role: 'system',
      payload: { text: 'You are a helpful assistant.' },
      metadata: { 'geppetto.middleware@v1': 'system-prompt-mw' },
    },
    {
      index: 1,
      kind: 'user',
      role: 'user',
      payload: { text: 'What is the weather in Paris?' },
      metadata: {},
    },
    {
      index: 2,
      id: 'tc_001',
      kind: 'tool_call',
      payload: { id: 'tc_001', name: 'get_weather', args: { location: 'Paris' } },
      metadata: { 'geppetto.inference_id@v1': 'inf_abc' },
    },
  ],
  metadata: { 
    'geppetto.session_id@v1': 'sess_01',
    'geppetto.inference_id@v1': 'inf_abc',
  },
  data: {},
};

const postToolsTurn: ParsedTurn = {
  id: 'turn_01',
  blocks: [
    {
      index: 0,
      kind: 'system',
      role: 'system',
      payload: { text: 'You are a helpful assistant.' },
      metadata: { 'geppetto.middleware@v1': 'system-prompt-mw' },
    },
    {
      index: 1,
      kind: 'user',
      role: 'user',
      payload: { text: 'What is the weather in Paris?' },
      metadata: {},
    },
    {
      index: 2,
      id: 'tc_001',
      kind: 'tool_call',
      payload: { id: 'tc_001', name: 'get_weather', args: { location: 'Paris' } },
      metadata: { 'geppetto.inference_id@v1': 'inf_abc' },
    },
    {
      index: 3,
      kind: 'tool_use',
      payload: { id: 'tc_001', result: { temperature: 18, condition: 'cloudy' } },
      metadata: { 'geppetto.tool_call_id@v1': 'tc_001' },
    },
  ],
  metadata: { 
    'geppetto.session_id@v1': 'sess_01',
    'geppetto.inference_id@v1': 'inf_abc',
    'geppetto.tools_executed@v1': ['get_weather'],
  },
  data: {},
};

const finalTurn: ParsedTurn = {
  id: 'turn_01',
  blocks: [
    {
      index: 0,
      kind: 'system',
      role: 'system',
      payload: { text: 'You are a helpful assistant.' },
      metadata: { 'geppetto.middleware@v1': 'system-prompt-mw' },
    },
    {
      index: 1,
      kind: 'user',
      role: 'user',
      payload: { text: 'What is the weather in Paris?' },
      metadata: {},
    },
    {
      index: 2,
      id: 'tc_001',
      kind: 'tool_call',
      payload: { id: 'tc_001', name: 'get_weather', args: { location: 'Paris' } },
      metadata: {},
    },
    {
      index: 3,
      kind: 'tool_use',
      payload: { id: 'tc_001', result: { temperature: 18, condition: 'cloudy' } },
      metadata: {},
    },
    {
      index: 4,
      kind: 'llm_text',
      role: 'assistant',
      payload: { text: 'The weather in Paris is currently 18°C and cloudy.' },
      metadata: { 'geppetto.inference_id@v1': 'inf_abc2' },
    },
  ],
  metadata: { 
    'geppetto.session_id@v1': 'sess_01',
    'geppetto.inference_id@v1': 'inf_abc2',
    'geppetto.total_usage@v1': { prompt_tokens: 1694, completion_tokens: 354 },
  },
  data: {},
};

export const PreToPost: Story = {
  args: {
    phaseA: 'pre_inference',
    phaseB: 'post_inference',
    turnA: preInferenceTurn,
    turnB: postInferenceTurn,
  },
};

export const PostToTools: Story = {
  args: {
    phaseA: 'post_inference',
    phaseB: 'post_tools',
    turnA: postInferenceTurn,
    turnB: postToolsTurn,
  },
};

export const ToolsToFinal: Story = {
  args: {
    phaseA: 'post_tools',
    phaseB: 'final',
    turnA: postToolsTurn,
    turnB: finalTurn,
  },
};

export const PreToFinal: Story = {
  args: {
    phaseA: 'pre_inference',
    phaseB: 'final',
    turnA: preInferenceTurn,
    turnB: finalTurn,
  },
};

export const NoChanges: Story = {
  args: {
    phaseA: 'pre_inference',
    phaseB: 'pre_inference',
    turnA: preInferenceTurn,
    turnB: preInferenceTurn,
  },
};

// Test reordering
const reorderedTurn: ParsedTurn = {
  id: 'turn_01',
  blocks: [
    {
      index: 0,
      kind: 'user',
      role: 'user',
      payload: { text: 'What is the weather in Paris?' },
      metadata: {},
    },
    {
      index: 1,
      kind: 'system',
      role: 'system',
      payload: { text: 'You are a helpful assistant.' },
      metadata: { 'geppetto.middleware@v1': 'system-prompt-mw' },
    },
  ],
  metadata: { 'geppetto.session_id@v1': 'sess_01' },
  data: {},
};

export const WithReorder: Story = {
  args: {
    phaseA: 'pre_inference',
    phaseB: 'post_inference',
    turnA: preInferenceTurn,
    turnB: reorderedTurn,
  },
};

// Test content change
const changedContentTurn: ParsedTurn = {
  id: 'turn_01',
  blocks: [
    {
      index: 0,
      kind: 'system',
      role: 'system',
      payload: { text: 'You are an EXPERT helpful assistant. Be very detailed.' },
      metadata: { 
        'geppetto.middleware@v1': 'system-prompt-mw',
        'geppetto.enhanced@v1': true,
      },
    },
    {
      index: 1,
      kind: 'user',
      role: 'user',
      payload: { text: 'What is the weather in Paris?' },
      metadata: {},
    },
  ],
  metadata: { 'geppetto.session_id@v1': 'sess_01' },
  data: {},
};

export const WithContentChange: Story = {
  args: {
    phaseA: 'pre_inference',
    phaseB: 'post_inference',
    turnA: preInferenceTurn,
    turnB: changedContentTurn,
  },
};
