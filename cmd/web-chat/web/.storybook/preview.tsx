import type { Preview } from '@storybook/react';
import React from 'react';
import { Provider } from 'react-redux';
import { MemoryRouter } from 'react-router-dom';
import { initialize, mswLoader } from 'msw-storybook-addon';
import { store as chatStore } from '../src/store/store';
import { store as debugStore } from '../src/debug-ui/store/store';
import '../src/debug-ui/index.css';
import '../src/webchat/styles/theme-default.css';
import '../src/webchat/styles/webchat.css';

initialize();

const preview: Preview = {
  loaders: [mswLoader],
  decorators: [
    (Story, context) => {
      if (context.title?.startsWith('Debug UI/')) {
        return (
          <Provider store={debugStore}>
            <MemoryRouter>
              <Story />
            </MemoryRouter>
          </Provider>
        );
      }

      return (
        <Provider store={chatStore}>
          <Story />
        </Provider>
      );
    },
  ],
  parameters: {
    layout: 'fullscreen',
  },
};

export default preview;
