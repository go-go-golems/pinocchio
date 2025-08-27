import { defineConfig } from 'vite';

export default defineConfig({
  base: './',
  build: {
    outDir: '../static/dist',
    emptyOutDir: true,
    sourcemap: true,
  },
  server: {
    port: 5173,
  },
});


