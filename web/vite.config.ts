import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import path from 'node:path'

// Vite config para Liz Web
// - Dev server en puerto 5173 (default Vite)
// - Proxy /api → http://localhost:3000 (backend Go)
// - Alias @ → /src
export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  server: {
    port: 5173,
    host: true,
    proxy: {
      '/api': {
        target: 'http://localhost:3000',
        changeOrigin: true,
        // SSE: NO buffer los chunks del pipeline de chat
        ws: false,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
    rollupOptions: {
      output: {
        manualChunks: {
          'react-vendor': ['react', 'react-dom'],
          'markdown-vendor': ['react-markdown', 'remark-gfm', 'react-syntax-highlighter'],
        },
      },
    },
  },
})
