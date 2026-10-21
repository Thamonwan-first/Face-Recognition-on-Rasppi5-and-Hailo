import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [react()],
  base: '/dashboard/',
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8443',
        changeOrigin: true,
      },
      '/auth': {
        target: 'http://localhost:8443',
        changeOrigin: true,
      },
      '/sync': {
        target: 'http://localhost:8443',
        changeOrigin: true,
      },
    },
  },
  build: {
    outDir: 'dist',
  },
});
