import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig({
  plugins: [react()],
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
