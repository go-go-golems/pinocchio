import type { Preview } from '@storybook/react';
import React from 'react';
import { Provider } from 'react-redux';
import { store as chatStore } from '../src/store/store';
import '../src/features/web-chat/styles/index.css';

const preview: Preview = {
  decorators: [
    (Story) => (
      <Provider store={chatStore}>
        <Story />
      </Provider>
    ),
  ],
  parameters: {
    layout: 'fullscreen',
  },
};

export default preview;
