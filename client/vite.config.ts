import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';

export default defineConfig({
  plugins: [react()],
  resolve: {
    conditions: ['import', 'module', 'browser', 'production'],
    alias: {
      '@humation/assets-humation-1': fileURLToPath(new URL('./node_modules/@humation/assets-humation-1/dist/index.js', import.meta.url)),
      '@humation/core': fileURLToPath(new URL('./node_modules/@humation/core/dist/index.js', import.meta.url)),
      '@humation/react': fileURLToPath(new URL('./node_modules/@humation/react/dist/index.js', import.meta.url)),
      'html-parse-stringify': fileURLToPath(new URL('./node_modules/html-parse-stringify/dist/html-parse-stringify.module.js', import.meta.url)),
      'playcaptcha/clawcaptcha.css': fileURLToPath(new URL('./node_modules/playcaptcha/dist/clawcaptcha.css', import.meta.url)),
      'playcaptcha': fileURLToPath(new URL('./node_modules/playcaptcha/dist/index.js', import.meta.url)),
      'use-sync-external-store/shim': fileURLToPath(new URL('./node_modules/use-sync-external-store/shim/index.js', import.meta.url)),
      'void-elements': fileURLToPath(new URL('./node_modules/void-elements/index.js', import.meta.url)),
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes('node_modules')) return undefined;
          if (/[\\/]node_modules[\\/](react|react-dom|scheduler)[\\/]/.test(id)) return 'react-vendor';
          if (/[\\/]node_modules[\\/]@astryxdesign[\\/]/.test(id)) return 'astryx-vendor';
          if (/[\\/]node_modules[\\/]lucide-react[\\/]/.test(id)) return 'icons-vendor';
          if (id.includes('i18next')) return 'i18n-vendor';
          return 'vendor';
        },
      },
    },
  },
  server: {
    port: 5173,
  },
});
