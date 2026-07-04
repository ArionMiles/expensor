import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { describe, it } from 'node:test'
import path from 'node:path'
import { fileURLToPath } from 'node:url'

const rootDir = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..')

describe('live screenshot workflow', () => {
  it('authenticates against the seeded component tenant before capture', async () => {
    const script = await readFile(
      path.join(rootDir, 'frontend', 'scripts', 'capture-live-screenshots.mjs'),
      'utf-8',
    )

    assert.match(script, /EXPENSOR_SCREENSHOT_BASE_URL/)
    assert.match(script, /EXPENSOR_SCREENSHOT_API_URL/)
    assert.match(script, /EXPENSOR_SCREENSHOT_EMAIL/)
    assert.match(script, /EXPENSOR_SCREENSHOT_PASSWORD/)
    assert.match(script, /john\.smith@example\.com/)
    assert.match(script, /context\.request\.get\(`\$\{apiURL\}\/api\/health`/)
    assert.match(script, /context\.request\.post\(`\$\{baseURL\}\/api\/session`/)
  })

  it('captures dashboard screenshots in all-time mode so seed dates do not age out', async () => {
    const script = await readFile(
      path.join(rootDir, 'frontend', 'scripts', 'capture-live-screenshots.mjs'),
      'utf-8',
    )

    assert.match(script, /\{ path: '\/\?summary=all_time', name: 'dashboard-light\.png', theme: 'light' \}/)
    assert.match(script, /\{ path: '\/\?summary=all_time', name: 'dashboard-dark\.png', theme: 'dark' \}/)
    assert.match(script, /window\.localStorage\.setItem\('expensor\.dashboard\.summaryMode', 'all_time'\)/)
  })

  it('provides review and capture tasks over the same seeded preview stack', async () => {
    const taskfile = await readFile(path.join(rootDir, 'Taskfile.yml'), 'utf-8')

    assert.match(taskfile, /screenshots:review:/)
    assert.match(taskfile, /docker compose -f tests\/component\/docker-compose\.yml run --quiet-pull --rm seed/)
    assert.match(taskfile, /npm run preview -- --host 127\.0\.0\.1 --port 4173/)
    assert.match(taskfile, /EXPENSOR_SCREENSHOT_BASE_URL=http:\/\/127\.0\.0\.1:4173/)
  })
})
