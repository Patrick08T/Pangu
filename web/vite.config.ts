import react from '@vitejs/plugin-react';
import { defineConfig } from 'vitest/config';
import {fileURLToPath} from 'node:url';

const proxy = {
  '/api': {
    target: 'http://127.0.0.1:8081',
  },
};

export default defineConfig({
  plugins: [react()],
  server: {
    host: '127.0.0.1',
    port: 5180,
    strictPort: true,
    fs: {allow:[fileURLToPath(new URL('.',import.meta.url)),fileURLToPath(new URL('../examples/templates',import.meta.url))]},
    proxy,
  },
  preview: { host: '127.0.0.1', port: 4173, strictPort: true, proxy },
  test: { environment: 'jsdom', globals: true },
});
