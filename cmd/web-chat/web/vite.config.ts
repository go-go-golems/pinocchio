import react from '@vitejs/plugin-react';
import { defineConfig, type Plugin } from 'vite';

function normalizePrefix(raw: string | undefined): string {
  const trimmed = (raw ?? '').trim();
  if (!trimmed || trimmed === '/') {
    return '';
  }
  const withLeadingSlash = trimmed.startsWith('/') ? trimmed : `/${trimmed}`;
  return withLeadingSlash.replace(/\/+$/, '');
}

function runtimeConfigPlugin(): Plugin {
  const basePrefix = normalizePrefix(process.env.VITE_WEBCHAT_BASE_PREFIX);
  const debugApiEnabled = process.env.VITE_WEBCHAT_DEBUG_API === '1' || process.env.VITE_WEBCHAT_DEBUG_API === 'true';
  const body = `window.__PINOCCHIO_WEBCHAT_CONFIG__ = ${JSON.stringify({ basePrefix, debugApiEnabled })};\n`;

  return {
    name: 'pinocchio-runtime-config',
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const pathname = (req.url ?? '').split('?')[0] ?? '';
        if (pathname === '/app-config.js' || pathname.endsWith('/app-config.js')) {
          res.statusCode = 200;
          res.setHeader('Content-Type', 'application/javascript; charset=utf-8');
          res.setHeader('Cache-Control', 'no-store');
          res.end(body);
          return;
        }
        next();
      });
    },
  };
}

export default defineConfig({
  plugins: [react(), runtimeConfigPlugin()],
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
      // Set VITE_WEBCHAT_BASE_PREFIX (e.g. /chat) to emit app-config.js
      // so the frontend can use the same root prefix as the Go app.
      // Note: Vite proxy contexts are prefix matches, not regex keys.
      '/chat': { target: process.env.VITE_BACKEND_ORIGIN ?? 'http://localhost:8080', changeOrigin: true },
      // Compatibility: some backend builds still expose timeline under /api/debug/timeline.
      '/api/timeline': {
        target: process.env.VITE_BACKEND_ORIGIN ?? 'http://localhost:8080',
        changeOrigin: true,
        // Preserve query string (e.g. ?conv_id=...) while remapping path.
        rewrite: (path) => path.replace(/^\/api\/timeline/, '/api/debug/timeline'),
      },
      '/api': { target: process.env.VITE_BACKEND_ORIGIN ?? 'http://localhost:8080', changeOrigin: true },
      '/ws': { target: process.env.VITE_BACKEND_ORIGIN ?? 'http://localhost:8080', ws: true, changeOrigin: true },
      '/hydrate': { target: process.env.VITE_BACKEND_ORIGIN ?? 'http://localhost:8080', changeOrigin: true },
    },
  },
});
