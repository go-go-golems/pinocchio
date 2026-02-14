import type { Meta, StoryObj } from '@storybook/react';
import { makeEventInspectorScenario } from '../mocks/scenarios';
import { EventInspector } from './EventInspector';

const meta: Meta<typeof EventInspector> = {
  title: 'Debug UI/EventInspector',
  component: EventInspector,
  parameters: {
    layout: 'padded',
  },
};

export default meta;
type Story = StoryObj<typeof EventInspector>;

export const LLMStart: Story = {
  args: makeEventInspectorScenario('llmStart').args,
};

export const LLMDelta: Story = {
  args: makeEventInspectorScenario('llmDelta').args,
};

export const LLMFinal: Story = {
  args: makeEventInspectorScenario('llmFinal').args,
};

export const ToolStart: Story = {
  args: makeEventInspectorScenario('toolStart').args,
};

export const ToolResult: Story = {
  args: makeEventInspectorScenario('toolResult').args,
};

export const WithCorrelatedNodes: Story = {
  args: makeEventInspectorScenario('withCorrelatedNodes').args,
};

export const WithTrustChecks: Story = {
  args: makeEventInspectorScenario('withTrustChecks').args,
};

export const WithFailedChecks: Story = {
  args: makeEventInspectorScenario('withFailedChecks').args,
};

export const FullExample: Story = {
  args: makeEventInspectorScenario('fullExample').args,
};
