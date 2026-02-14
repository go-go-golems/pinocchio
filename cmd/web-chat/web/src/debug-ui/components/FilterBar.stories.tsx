import type { Meta, StoryObj } from '@storybook/react';
import { useState } from 'react';
import { FilterBar, type FilterState } from './FilterBar';

const meta: Meta<typeof FilterBar> = {
  title: 'Debug UI/FilterBar',
  component: FilterBar,
  parameters: {
    layout: 'padded',
  },
  decorators: [
    (Story) => (
      <div style={{ maxWidth: '320px' }}>
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof FilterBar>;

const emptyFilters: FilterState = {
  blockKinds: [],
  eventTypes: [],
  searchQuery: '',
  showEmpty: true,
};

const someFilters: FilterState = {
  blockKinds: ['user', 'llm_text', 'tool_call'],
  eventTypes: ['llm.start', 'llm.final'],
  searchQuery: '',
  showEmpty: true,
};

const searchFilters: FilterState = {
  blockKinds: [],
  eventTypes: [],
  searchQuery: 'weather',
  showEmpty: false,
};

export const Empty: Story = {
  args: {
    filters: emptyFilters,
    onFiltersChange: () => {},
  },
};

export const WithSomeFilters: Story = {
  args: {
    filters: someFilters,
    onFiltersChange: () => {},
  },
};

export const WithSearch: Story = {
  args: {
    filters: searchFilters,
    onFiltersChange: () => {},
  },
};

export const WithCloseButton: Story = {
  args: {
    filters: emptyFilters,
    onFiltersChange: () => {},
    onClose: () => alert('Close clicked'),
  },
};

// Interactive story
function InteractiveFilterBar() {
  const [filters, setFilters] = useState<FilterState>(emptyFilters);
  
  return (
    <div>
      <FilterBar filters={filters} onFiltersChange={setFilters} />
      <div style={{ marginTop: '16px', padding: '12px', background: 'var(--bg-card)', borderRadius: '6px' }}>
        <h4 style={{ marginBottom: '8px', fontSize: '12px', color: 'var(--text-muted)' }}>Current State:</h4>
        <pre style={{ fontSize: '11px' }}>{JSON.stringify(filters, null, 2)}</pre>
      </div>
    </div>
  );
}

export const Interactive: Story = {
  render: () => <InteractiveFilterBar />,
};

export const AllSelected: Story = {
  args: {
    filters: {
      blockKinds: ['system', 'user', 'llm_text', 'tool_call', 'tool_use', 'reasoning'],
      eventTypes: ['llm.start', 'llm.delta', 'llm.final', 'tool.start', 'tool.result', 'tool.done', 'log'],
      searchQuery: 'test query',
      showEmpty: false,
    },
    onFiltersChange: () => {},
  },
};
