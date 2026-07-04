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

  it('captures dashboard screenshots in current-month mode and waits for the daemon status', async () => {
    const script = await readFile(
      path.join(rootDir, 'frontend', 'scripts', 'capture-live-screenshots.mjs'),
      'utf-8',
    )

    assert.match(script, /\{ path: '\/', name: 'dashboard-light\.png', theme: 'light' \}/)
    assert.match(script, /\{ path: '\/', name: 'dashboard-dark\.png', theme: 'dark' \}/)
    assert.doesNotMatch(script, /summary=all_time/)
    assert.doesNotMatch(script, /expensor\.dashboard\.summaryMode/)
    assert.match(script, /context\.request\.post\(`\$\{baseURL\}\/api\/daemon\/start`/)
    assert.match(script, /page\.getByText\('daemon running'\)/)
  })

  it('keeps seeded screenshot transactions in the current month without calendar-month edits', async () => {
    const seed = await readFile(
      path.join(rootDir, 'tests', 'component', 'fixtures', 'seed.sql'),
      'utf-8',
    )

    assert.match(seed, /date_trunc\('month', NOW\(\) AT TIME ZONE 'Asia\/Kolkata'\)/)
    assert.doesNotMatch(seed, /make_timestamptz\(\s*2026,\s*5,/)
    assert.doesNotMatch(seed, /DATE '2025-06-01'/)
    assert.match(
      seed,
      /current_month\.month_start - \(\(\(\(seq - 76\) \/ 15\) \+ 1\) \* INTERVAL '1 month'\)/,
    )
  })

  it('provides review and capture tasks over the same seeded preview stack', async () => {
    const taskfile = await readFile(path.join(rootDir, 'Taskfile.yml'), 'utf-8')

    assert.match(taskfile, /screenshots:review:/)
    assert.match(
      taskfile,
      /docker compose -f tests\/component\/docker-compose\.yml run --quiet-pull --rm seed/,
    )
    assert.match(taskfile, /npm run preview -- --host 127\.0\.0\.1 --port 4173/)
    assert.match(taskfile, /EXPENSOR_SCREENSHOT_BASE_URL=http:\/\/127\.0\.0\.1:4173/)
  })
})
