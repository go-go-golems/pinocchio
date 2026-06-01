import type { Preview } from '@storybook/react';
import React from 'react';
import { Provider } from 'react-redux';
import { initialize, mswLoader } from 'msw-storybook-addon';
import { store as chatStore } from '../src/store/store';
import '../src/features/web-chat/styles/index.css';

initialize();

const preview: Preview = {
  loaders: [mswLoader],
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
