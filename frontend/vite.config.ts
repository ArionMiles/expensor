import react from '@vitejs/plugin-react'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { defineConfig } from 'vite'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const apiProxyTarget = process.env.VITE_API_PROXY_TARGET ?? 'http://localhost:8080'
const avatarContentDir = path.resolve(__dirname, './content/avatars')

export default defineConfig({
  plugins: [react()],
  server: {
    fs: {
      allow: [__dirname, avatarContentDir],
    },
    proxy: {
      '/api': apiProxyTarget,
    },
  },
  preview: {
    proxy: {
      '/api': apiProxyTarget,
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@avatar-content': avatarContentDir,
    },
  },
})
