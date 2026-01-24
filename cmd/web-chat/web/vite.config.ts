import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: '../static/dist',
    emptyOutDir: true,
    sourcemap: true,
  },
  server: {
    port: 5173,
    proxy: {
      // Development convenience: forward API/WS to the Go server.
      // Adjust backend port via VITE_BACKEND_ORIGIN if needed.
      '^/(chat|ws|hydrate|default|agent)(/.*)?$': {
        target: process.env.VITE_BACKEND_ORIGIN ?? 'http://localhost:8080',
        ws: true,
        changeOrigin: true,
      },
    },
  },
});

