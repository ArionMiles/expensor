import { configDefaults, defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const avatarContentDir = path.resolve(__dirname, './content/avatars')

export default defineConfig({
  plugins: [react()],
  server: {
    fs: {
      allow: [__dirname, avatarContentDir],
    },
  },
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
      '@avatar-content': avatarContentDir,
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    globals: true,
    css: true,
    exclude: [
      ...configDefaults.exclude,
      'playwright/**',
      'playwright.config.ts',
      'playwright.global.setup.ts',
    ],
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html', 'lcov', 'json-summary'],
      reportsDirectory: './coverage',
      include: ['src/**/*.{ts,tsx}'],
      exclude: ['src/main.tsx', 'src/test/**', 'src/mocks/**'],
    },
  },
})
