# Testing Phase 1 Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land baseline backend unit coverage reporting plus the frontend unit/component test harness and the first stable frontend regression tests.

**Architecture:** Phase 1 is the lowest-risk infrastructure phase. It adds native Go unit coverage, introduces Vitest/RTL/MSW to the frontend, and implements the initial high-value UI tests identified in Phase 0. It does not attempt OpenAPI, backend component harnesses, or Playwright execution yet.

**Tech Stack:** Go coverage, Vitest, React Testing Library, user-event, jsdom, MSW, Vite, Task, GitHub Actions

---

## Status

- Status: Complete
- Last updated: 2026-05-19
- Owner: Unassigned
- Execution note: Frontend harness, initial component/page tests, Task targets, CI wiring, and backend coverage reporting are implemented. The workstream is ready to hand off to Phase 2.

## Files

- Modify: `Taskfile.yml`
- Modify: `.github/workflows/ci.yml`
- Modify: `frontend/package.json`
- Create: `frontend/vitest.config.ts`
- Create: `frontend/src/test/setup.ts`
- Create: `frontend/src/test/render.tsx`
- Create: `frontend/src/test/server.ts`
- Create: `frontend/src/test/handlers/index.ts`
- Create: `frontend/src/test/fixtures/`
- Create: `frontend/src/components/InlineSelect.test.tsx`
- Create: `frontend/src/components/LabelCombobox.test.tsx`
- Create: `frontend/src/components/DateRangePicker.test.tsx`
- Create: `frontend/src/components/ConfirmModal.test.tsx`
- Create: `frontend/src/components/SlideNotification.test.tsx`
- Create: `frontend/src/pages/Transactions.test.tsx`
- Create: `frontend/src/pages/Settings.test.tsx`

## Planned File Responsibilities

- `frontend/package.json`
  Purpose: add frontend test and coverage scripts plus dev dependencies.
- `frontend/vitest.config.ts`
  Purpose: central Vitest config for jsdom, path resolution, coverage, and setup file.
- `frontend/src/test/setup.ts`
  Purpose: global test setup, cleanup, MSW lifecycle, matchMedia/localStorage/polyfill stubs, fake timer helpers where needed.
- `frontend/src/test/render.tsx`
  Purpose: app-aware render helper with `QueryClientProvider`, `MemoryRouter`, `DisplayProvider`, and resettable test client.
- `frontend/src/test/server.ts`
  Purpose: MSW server bootstrap for Vitest.
- `frontend/src/test/handlers/index.ts`
  Purpose: default API handlers for status, settings, transactions, labels, categories, buckets, and no-op mutation endpoints needed by the first page tests.
- `Taskfile.yml`
  Purpose: add `test:fe`, `test:fe:watch`, `test:fe:cover`, and coverage-friendly backend targets.
- `.github/workflows/ci.yml`
  Purpose: add frontend unit test job and backend/frontend coverage artifacts.
- component/page test files
  Purpose: implement the initial regression tests selected in Phase 0.

---

### Task 1: Add Frontend Test Dependencies and Scripts

**Files:**
- Modify: `frontend/package.json`

- [x] **Step 1: Write the failing install check**

Run:

```bash
cd frontend && npm run test
```

Expected: fail because no `test` script exists yet.

- [x] **Step 2: Add the required frontend test scripts and dev dependencies**

Update `frontend/package.json` so `scripts` includes:

```json
"test": "vitest run",
"test:watch": "vitest",
"test:cover": "vitest run --coverage"
```

Add dev dependencies for:

```json
"vitest": "^3.2.4",
"jsdom": "^26.1.0",
"@testing-library/react": "^16.3.0",
"@testing-library/user-event": "^14.6.1",
"@testing-library/jest-dom": "^6.6.3",
"msw": "^2.11.2",
"@vitest/coverage-v8": "^3.2.4"
```

- [x] **Step 3: Install dependencies**

Run:

```bash
cd frontend && npm install
```

Expected: lockfile updates and new test packages installed.

- [x] **Step 4: Verify the new scripts exist**

Run:

```bash
cd frontend && npm run test -- --help
```

Expected: Vitest CLI help output instead of “missing script”.

- [x] **Step 5: Commit**

```bash
git add frontend/package.json frontend/package-lock.json
git commit --no-gpg-sign -m "test: add frontend test dependencies"
```

---

### Task 2: Add Vitest Configuration and Shared Test Harness

**Files:**
- Create: `frontend/vitest.config.ts`
- Create: `frontend/src/test/setup.ts`
- Create: `frontend/src/test/render.tsx`
- Create: `frontend/src/test/server.ts`
- Create: `frontend/src/test/handlers/index.ts`

- [x] **Step 1: Write the failing config check**

Run:

```bash
test -f frontend/vitest.config.ts && test -f frontend/src/test/setup.ts && test -f frontend/src/test/render.tsx && test -f frontend/src/test/server.ts && test -f frontend/src/test/handlers/index.ts
```

Expected: command exits non-zero because the files do not exist yet.

- [x] **Step 2: Create `frontend/vitest.config.ts`**

Write:

```ts
import { defineConfig } from 'vitest/config'
import react from '@vitejs/plugin-react'
import path from 'node:path'

export default defineConfig({
  plugins: [react()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    globals: true,
    css: true,
    coverage: {
      provider: 'v8',
      reporter: ['text', 'html', 'lcov'],
      reportsDirectory: './coverage',
      exclude: ['src/main.tsx', 'src/test/**'],
    },
  },
})
```

- [x] **Step 3: Create `frontend/src/test/server.ts`**

Write:

```ts
import { setupServer } from 'msw/node'
import { handlers } from './handlers'

export const server = setupServer(...handlers)
```

- [x] **Step 4: Create `frontend/src/test/setup.ts`**

Write:

```ts
import '@testing-library/jest-dom/vitest'
import { afterAll, afterEach, beforeAll, vi } from 'vitest'
import { cleanup } from '@testing-library/react'
import { server } from './server'

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  configurable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }),
})

window.HTMLElement.prototype.scrollIntoView = vi.fn()

beforeAll(() => {
  server.listen({ onUnhandledRequest: 'error' })
})

afterEach(() => {
  cleanup()
  server.resetHandlers()
  vi.restoreAllMocks()
  localStorage.clear()
})

afterAll(() => {
  server.close()
})
```

- [x] **Step 5: Create `frontend/src/test/render.tsx`**

Write:

```tsx
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import type { ReactElement } from 'react'
import { DisplayProvider } from '@/contexts/DisplayContext'

export function createTestQueryClient() {
  return new QueryClient({
    defaultOptions: {
      queries: {
        retry: false,
        staleTime: 0,
        gcTime: Infinity,
      },
      mutations: {
        retry: false,
      },
    },
  })
}

export function renderWithProviders(
  ui: ReactElement,
  { route = '/' }: { route?: string } = {},
) {
  const queryClient = createTestQueryClient()

  return {
    queryClient,
    ...render(
      <QueryClientProvider client={queryClient}>
        <DisplayProvider>
          <MemoryRouter initialEntries={[route]}>{ui}</MemoryRouter>
        </DisplayProvider>
      </QueryClientProvider>,
    ),
  }
}
```

- [x] **Step 6: Create `frontend/src/test/handlers/index.ts`**

Write a default handler set like:

```ts
import { http, HttpResponse } from 'msw'

export const handlers = [
  http.get('/api/status', () => HttpResponse.json({ daemon: { running: false } })),
  http.get('/api/version', () => HttpResponse.json({ version: 'test' })),
  http.get('/api/config/timezone', () => HttpResponse.json({ timezone: 'UTC' })),
  http.put('/api/config/timezone', async () => HttpResponse.json({ timezone: 'UTC' })),
  http.get('/api/config/time-format', () => HttpResponse.json({ time_format: '24h' })),
  http.put('/api/config/time-format', async () => HttpResponse.json({ time_format: '24h' })),
  http.get('/api/config/base-currency', () => HttpResponse.json({ base_currency: 'USD' })),
  http.put('/api/config/base-currency', async () => HttpResponse.json({ base_currency: 'USD' })),
  http.get('/api/config/scan-interval', () => HttpResponse.json({ scan_interval: '60' })),
  http.put('/api/config/scan-interval', async () => HttpResponse.json({ scan_interval: '60' })),
  http.get('/api/config/lookback-days', () => HttpResponse.json({ lookback_days: '180' })),
  http.put('/api/config/lookback-days', async () => HttpResponse.json({ lookback_days: '180' })),
  http.get('/api/config/active-reader', () => HttpResponse.json({ reader: '' })),
  http.get('/api/config/readers/:reader/checkpoint', () =>
    HttpResponse.json({ last_scan_at: null }),
  ),
  http.delete('/api/config/readers/:reader/checkpoint', () => new HttpResponse(null, { status: 204 })),
  http.post('/api/daemon/rescan', async () => HttpResponse.json({ status: 'queued' })),
  http.get('/api/config/sync/status', () =>
    HttpResponse.json({ last_synced_at: null, error: null, entries_updated: 0 }),
  ),
  http.post('/api/config/sync', async () => HttpResponse.json({ status: 'queued' })),
  http.get('/api/config/labels', () => HttpResponse.json([])),
  http.get('/api/config/labels/mappings', () => HttpResponse.json({})),
  http.post('/api/config/labels', async () => HttpResponse.json({ name: 'test', color: '#000000' })),
  http.put('/api/config/labels/:name', async () =>
    HttpResponse.json({ name: 'test', color: '#000000' }),
  ),
  http.delete('/api/config/labels/:name', () => new HttpResponse(null, { status: 204 })),
  http.post('/api/config/labels/:name/apply', async () => HttpResponse.json({ applied: 0 })),
  http.delete('/api/config/labels/:name/merchant', () =>
    HttpResponse.json({ removed: 0 }),
  ),
  http.get('/api/config/categories', () => HttpResponse.json([])),
  http.post('/api/config/categories', async () => HttpResponse.json({ name: 'test' })),
  http.delete('/api/config/categories/:name', () => new HttpResponse(null, { status: 204 })),
  http.get('/api/config/buckets', () => HttpResponse.json([])),
  http.post('/api/config/buckets', async () => HttpResponse.json({ name: 'test' })),
  http.delete('/api/config/buckets/:name', () => new HttpResponse(null, { status: 204 })),
  http.get('/api/config/banks', () => HttpResponse.json([])),
  http.get('/api/transactions/facets', () =>
    HttpResponse.json({ sources: [], categories: [], currencies: [], labels: [], buckets: [] }),
  ),
  http.get('/api/transactions', () =>
    HttpResponse.json({ transactions: [], total: 0, total_amount: 0, base_currency: 'USD' }),
  ),
]
```

- [x] **Step 7: Verify the harness files exist**

Run:

```bash
test -f frontend/vitest.config.ts && test -f frontend/src/test/setup.ts && test -f frontend/src/test/render.tsx && test -f frontend/src/test/server.ts && test -f frontend/src/test/handlers/index.ts
```

Expected: command exits 0.

- [x] **Step 8: Commit**

```bash
git add frontend/vitest.config.ts frontend/src/test
git commit --no-gpg-sign -m "test: add frontend test harness"
```

---

### Task 3: Add Taskfile Targets for Frontend Tests and Better Backend Coverage

**Files:**
- Modify: `Taskfile.yml`

- [x] **Step 1: Add frontend test targets**

Add tasks:

```yaml
  test:fe:
    summary: Run frontend unit and component tests.
    desc: >-
      Executes the Vitest suite in non-watch mode for the frontend app.
      Intended for local verification and CI.
    dir: frontend
    deps: [frontend:install]
    cmd: npm run test

  test:fe:watch:
    summary: Run frontend tests in watch mode.
    desc: >-
      Starts Vitest in watch mode for iterative frontend test development.
      Intended for local development only.
    dir: frontend
    deps: [frontend:install]
    cmd: npm run test:watch

  test:fe:cover:
    summary: Run frontend tests with coverage output.
    desc: >-
      Executes the frontend Vitest suite with V8 coverage enabled and writes
      coverage artifacts under the frontend coverage directory.
    dir: frontend
    deps: [frontend:install]
    cmd: npm run test:cover
```

- [x] **Step 2: Tighten backend coverage target for CI artifact use**

Adjust `test:be:cover` to generate a stable text profile without requiring HTML-only output:

```yaml
  test:be:cover:
    summary: Run Go unit tests with coverage report.
    desc: >-
      Executes backend Go unit tests with coverage profiling enabled and prints
      the function-level coverage summary for CI and local inspection.
    dir: backend
    cmd: go test -v -coverprofile=coverage.out ./... && go tool cover -func=coverage.out
```

- [x] **Step 3: Update aggregate `test` target**

Change `test` to:

```yaml
  test:
    summary: Run all backend and frontend unit/component tests.
    desc: >-
      Executes the default backend Go test suite and the frontend Vitest suite.
      Excludes later-phase component, contract, and browser test harnesses.
    cmds:
      - task: test:be
      - task: test:fe
```

- [x] **Step 4: Verify the new task names parse**

Run:

```bash
task --list | rg "test:fe|test:fe:watch|test:fe:cover|test:be:cover"
```

Expected: all four tasks appear in the list.

- [x] **Step 4a: Verify task metadata is descriptive**

Run:

```bash
sed -n '60,130p' Taskfile.yml
```

Expected: each newly added or updated test target includes a clear `summary`, and
where appropriate a non-trivial `desc` that explains local vs CI intent and output.

- [x] **Step 5: Commit**

```bash
git add Taskfile.yml
git commit --no-gpg-sign -m "test: add phase 1 task targets"
```

---

### Task 4: Add Initial Component Primitive Tests

**Files:**
- Create: `frontend/src/components/InlineSelect.test.tsx`
- Create: `frontend/src/components/LabelCombobox.test.tsx`
- Create: `frontend/src/components/DateRangePicker.test.tsx`
- Create: `frontend/src/components/ConfirmModal.test.tsx`
- Create: `frontend/src/components/SlideNotification.test.tsx`

- [x] **Step 1: Add `InlineSelect` tests**

Write tests that cover:

- keyboard navigation with arrow keys and enter
- outside click closes dropdown
- selecting the current value does not call `onCommit`

- [x] **Step 2: Add `ConfirmModal` tests**

Write tests that cover:

- confirm callback
- cancel callback
- escape closes modal
- overlay mouse-down closes modal

- [x] **Step 3: Add `SlideNotification` tests**

Write tests that cover:

- clicking actions calls `onAction`
- timeout triggers the default action
- use fake timers to avoid flake

- [x] **Step 4: Add `DateRangePicker` tests**

Write tests that cover:

- opening and closing the picker
- clear action
- apply disabled until `from` exists
- selected range text updates after applying

- [x] **Step 5: Add `LabelCombobox` tests**

Write tests that cover:

- add button opens input
- filtering narrows visible labels
- selecting existing label triggers mutation path
- create affordance appears for new labels

- [x] **Step 6: Run the component test subset**

Run:

```bash
cd frontend && npx vitest run src/components/InlineSelect.test.tsx src/components/ConfirmModal.test.tsx src/components/SlideNotification.test.tsx src/components/DateRangePicker.test.tsx src/components/LabelCombobox.test.tsx
```

Expected: all component tests pass.

- [x] **Step 7: Commit**

```bash
git add frontend/src/components/*.test.tsx
git commit --no-gpg-sign -m "test: add component primitive tests"
```

---

### Task 5: Add Initial Page Tests for Transactions and Settings

**Files:**
- Create: `frontend/src/pages/Transactions.test.tsx`
- Create: `frontend/src/pages/Settings.test.tsx`
- Modify: `frontend/src/test/handlers/index.ts`

- [x] **Step 1: Extend MSW handlers for the page test data**

Add deterministic handlers for:

- transactions list with seeded rows
- facets response with labels/categories/buckets
- settings configuration endpoints used by `Settings.tsx`

- [x] **Step 2: Add `Settings` page tests**

Write tests that cover:

- `tab` query param controls the selected tab
- clicking a tab updates the rendered section
- invalid/missing `tab` falls back to `general`

- [x] **Step 3: Add `Transactions` page tests**

Write tests that cover:

- page renders with mocked transactions and facets
- search/filter state round-trips through URL query params
- initial URL state restores the expected filter UI

- [x] **Step 4: Run the page test subset**

Run:

```bash
cd frontend && npx vitest run src/pages/Settings.test.tsx src/pages/Transactions.test.tsx
```

Expected: page tests pass with MSW-backed responses.

- [x] **Step 5: Commit**

```bash
git add frontend/src/pages/*.test.tsx frontend/src/test/handlers/index.ts
git commit --no-gpg-sign -m "test: add initial page regression tests"
```

---

### Task 6: Wire Coverage and Frontend Tests into CI

**Files:**
- Modify: `.github/workflows/ci.yml`

- [x] **Step 1: Add frontend unit test job**

Add a `test-frontend` job that:

- checks out the repo
- sets up Node 20
- installs Task
- runs `npm ci` in `frontend`
- runs `task test:fe`
- runs `task test:fe:cover`
- uploads `frontend/coverage` as an artifact

- [x] **Step 2: Add backend coverage artifact step**

Extend the backend test job to run:

```yaml
- name: Run backend coverage
  run: task test:be:cover

- name: Upload backend coverage artifact
  uses: actions/upload-artifact@v4
  with:
    name: backend-coverage
    path: backend/coverage.out
```

- [x] **Step 3: Verify workflow syntax**

Run:

```bash
git diff -- .github/workflows/ci.yml
```

Expected: frontend unit testing and backend/frontend coverage artifact steps are present and readable.

- [x] **Step 4: Commit**

```bash
git add .github/workflows/ci.yml
git commit --no-gpg-sign -m "ci: add frontend test and coverage jobs"
```

---

### Task 7: Final Verification and Phase Status Reflection

**Files:**
- Verify: `frontend/package.json`
- Verify: `frontend/vitest.config.ts`
- Verify: `frontend/src/test/*`
- Verify: `frontend/src/components/*.test.tsx`
- Verify: `frontend/src/pages/*.test.tsx`
- Verify: `Taskfile.yml`
- Verify: `.github/workflows/ci.yml`
- Modify: `docs/superpowers/specs/2026-05-19-frontend-testing.md`
- Modify: `docs/superpowers/plans/2026-05-19-testing-program.md`
- Modify: `docs/superpowers/plans/2026-05-19-testing-phase-1-coverage-frontend-unit.md`

- [x] **Step 1: Run frontend suite**

Run:

```bash
task test:fe
```

Expected: frontend tests pass.

- [x] **Step 2: Run frontend coverage**

Run:

```bash
task test:fe:cover
```

Expected: frontend coverage report is generated under `frontend/coverage`.

- [x] **Step 3: Run backend coverage**

Run:

```bash
task test:be:cover
```

Expected: `backend/coverage.out` exists and coverage summary is printed.

- [x] **Step 4: Mark the Phase 1 workstream complete in the tracking docs**

Change:

```md
| Phase 1 — Coverage plumbing and frontend unit harness | Plan Ready | `docs/superpowers/plans/2026-05-19-testing-phase-1-coverage-frontend-unit.md` |
```

to:

```md
| Phase 1 — Coverage plumbing and frontend unit harness | Complete | `docs/superpowers/plans/2026-05-19-testing-phase-1-coverage-frontend-unit.md` |
```

Update the program index entry to:

```md
- [x] Phase 1: Coverage plumbing and frontend unit harness
  Status: Complete
```

Update this plan’s status block to:

```md
- Status: Complete
```

- [x] **Step 5: Review the final diff**

Run:

```bash
git diff -- frontend Taskfile.yml .github/workflows/ci.yml docs/superpowers/specs/2026-05-19-frontend-testing.md docs/superpowers/plans/2026-05-19-testing-program.md docs/superpowers/plans/2026-05-19-testing-phase-1-coverage-frontend-unit.md
```

Expected: diff is limited to the Phase 1 harness, tests, CI wiring, and status updates.

- [x] **Step 6: Commit the phase completion**

```bash
git add frontend Taskfile.yml .github/workflows/ci.yml docs/superpowers/specs/2026-05-19-frontend-testing.md docs/superpowers/plans/2026-05-19-testing-program.md docs/superpowers/plans/2026-05-19-testing-phase-1-coverage-frontend-unit.md
git commit --no-gpg-sign -m "test: complete phase 1 frontend harness"
```

---

## Self-Review

### Spec Coverage

- Covers the full Phase 1 scope: backend unit coverage, frontend harness, initial frontend tests, and CI artifacts.
- Uses the exact initial frontend targets chosen in the Phase 0 inventory.

### Placeholder Scan

- No `TODO` or `TBD` placeholders remain.
- All tasks include exact file paths, commands, and explicit intended outputs.

### Type Consistency

- `test:fe`, `test:fe:watch`, and `test:fe:cover` are used consistently throughout.
- Status terms line up with the top-level program spec.

## Exit Criteria

- `task test:fe` and `task test:fe:cover` exist and pass
- frontend unit harness exists under `frontend/src/test`
- initial component/page regression tests exist for the Phase 0 targets
- backend unit coverage is generated locally and in CI
- frontend coverage artifacts are generated locally and in CI

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-19-testing-phase-1-coverage-frontend-unit.md`. Two execution options:

**1. Subagent-Driven (recommended)** - I dispatch a fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?
