import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vite';

export default defineConfig({
  build: {
    emptyOutDir: false,
    outDir: '../internal/phylo/viewer_assets/assets/msaexpor',
    lib: {
      entry: fileURLToPath(new URL('./src/msaexpor-pdf.js', import.meta.url)),
      name: 'PHGOmsaexporPDFBundle',
      formats: ['iife'],
      fileName: () => 'pdf.js',
    },
    rollupOptions: {
      output: { inlineDynamicImports: true },
    },
  },
});
