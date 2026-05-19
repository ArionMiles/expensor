import { access } from 'node:fs/promises'
import { fileURLToPath } from 'node:url'
import path from 'node:path'

export default async function globalSetup() {
  const rootDir = path.dirname(fileURLToPath(import.meta.url))
  const workerPath = path.resolve(rootDir, 'public', 'mockServiceWorker.js')

  try {
    await access(workerPath)
  } catch {
    throw new Error(
      'MSW browser worker is missing at frontend/public/mockServiceWorker.js. Run `npx msw init public --no-save` from frontend/.',
    )
  }
}
