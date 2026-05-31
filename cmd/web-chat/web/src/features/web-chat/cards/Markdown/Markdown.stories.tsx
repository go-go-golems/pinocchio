import type { Meta, StoryObj } from '@storybook/react';
import { CardStoryFrame } from '../storyDecorators';
import { Markdown } from './Markdown';

const meta: Meta<typeof Markdown> = {
  title: 'WebChat/Cards/Markdown',
  component: Markdown,
  decorators: [(Story) => <CardStoryFrame><Story /></CardStoryFrame>],
};

export default meta;
type Story = StoryObj<typeof Markdown>;

export const Links: Story = {
  args: { text: 'Read the [Pinocchio docs](https://github.com/go-go-golems/pinocchio).' },
};

export const CodeBlocks: Story = {
  args: { text: 'Run:\n\n```bash\nnpm run typecheck\n```' },
};

export const Lists: Story = {
  args: { text: '- First item\n- Second item\n  - Nested item' },
};

export const UnsafeUrl: Story = {
  args: { text: 'This [unsafe link](javascript:alert(1)) should not become a clickable anchor.' },
};
