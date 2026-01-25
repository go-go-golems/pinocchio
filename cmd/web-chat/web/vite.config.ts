import react from '@vitejs/plugin-react';
import { defineConfig } from 'vite';

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
      // Note: Vite proxy contexts are prefix matches, not regex keys.
      '/chat': { target: process.env.VITE_BACKEND_ORIGIN ?? 'http://localhost:8080', changeOrigin: true },
      '/ws': { target: process.env.VITE_BACKEND_ORIGIN ?? 'http://localhost:8080', ws: true, changeOrigin: true },
      '/hydrate': { target: process.env.VITE_BACKEND_ORIGIN ?? 'http://localhost:8080', changeOrigin: true },
      '/timeline': { target: process.env.VITE_BACKEND_ORIGIN ?? 'http://localhost:8080', changeOrigin: true },
      // Optional convenience endpoints:
      '/planning': { target: process.env.VITE_BACKEND_ORIGIN ?? 'http://localhost:8080', changeOrigin: true },
    },
  },
});
