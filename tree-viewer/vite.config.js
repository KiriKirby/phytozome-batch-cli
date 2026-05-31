import { fileURLToPath, URL } from 'node:url';
import { copyFileSync, mkdirSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

function phgoIconPlugin() {
  return {
    name: 'phgo-viewer-icon',
    closeBundle() {
      const source = fileURLToPath(new URL('../docs/logo2.png', import.meta.url));
      const target = resolve(fileURLToPath(new URL('.', import.meta.url)), 'dist', 'phgo-icon.png');
      mkdirSync(dirname(target), { recursive: true });
      copyFileSync(source, target);
    },
  };
}

export default defineConfig({
  plugins: [react(), phgoIconPlugin()],
  build: {
    rollupOptions: {
      output: {
        entryFileNames: 'assets/[name].js',
        chunkFileNames: 'assets/[name].js',
        assetFileNames: 'assets/[name][extname]',
      },
    },
  },
  resolve: {
    alias: [
      {
        find: /^reactreejs$/,
        replacement: fileURLToPath(new URL('./node_modules/reactreejs/dist/index.mjs', import.meta.url)),
      },
    ],
  },
});
