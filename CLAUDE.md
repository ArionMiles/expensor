# Expensor — Claude Instructions

## Project Overview

Expensor is a personal finance tracker that reads expense emails (Gmail, Thunderbird), extracts transaction details via regex rules, and stores them in PostgreSQL. It has a Go backend (HTTP API + daemon runner) and a React/Vite/Tailwind frontend.

## Repository Layout

```
backend/          Go module (github.com/ArionMiles/expensor/backend)
  cmd/server/     Main binary — HTTP server + daemon orchestration
  internal/
    api/          HTTP handlers, routing, middleware, Storer interface
    daemon/       Reader → writer pipeline
    plugins/      Plugin registry
    store/        PostgreSQL query layer (pgx/v5)
  migrations/     Numbered SQL files embedded into the binary (001_, 002_, …)
  pkg/
    api/          Core interfaces: Reader, Writer, Rule, Labels
    config/       koanf-based env config
    extractor/    Regex extraction helpers
    plugins/      Plugin wrappers (readers: gmail, thunderbird; writer: postgres)
    reader/       Concrete reader implementations
    writer/       Concrete writer implementations
frontend/         React + Vite + Tailwind (src/, public/)
tests/            Integration test helpers, local docker-compose
```

## Essential Commands

Always prefer `task` over bare `go`/`npm` commands — task targets handle tooling, env loading, and working directory.

```bash
task dev              # Start postgres + backend + frontend (full stack, loads tests/.env)
task run              # Backend only
task run:frontend     # Frontend Vite dev server only

# Formatting (aggregate runs both stacks)
task fmt              # Format all (Go: gci + gofumpt; TS: prettier)
task fmt:be           # Format Go only
task fmt:fe           # Format frontend only (prettier)

# Linting (aggregate runs both stacks)
task lint             # Lint all (Go local config + TypeScript)
task lint:be          # Lint Go with local config (.golangci.toml)
task lint:be:prod     # Strict CI lint (.golangci-prod.toml) — must be clean before commit
task lint:be:new      # Lint only new/changed Go files (compared to main)
task lint:fe          # TypeScript type-check (tsc --noEmit)

# Testing
task test             # Run all tests
task test:be          # Go unit tests (integration tests require Docker; -short skips them)
task test:be:cover    # Go tests with HTML coverage report
task test:be:component # Docker Compose-backed backend component tests
task test:be:contract # Backend OpenAPI contract tests via Schemathesis
task test:fe          # Frontend unit and component tests (Vitest)
task test:fe:e2e      # Frontend mocked Playwright E2E tests
task test:fe:e2e:smoke # Full-stack Playwright smoke tests against backend + Postgres
task screenshots:readme # Mocked README dashboard screenshot fixture
task screenshots:live   # Live high-resolution dashboard + transactions screenshots

# Security audit (aggregate runs both stacks)
task audit            # Audit all (Go: govulncheck; npm: npm audit)
task audit:be         # govulncheck on Go source
task audit:fe         # npm audit on production dependencies

task build:binary     # Optimized Go binary → bin/expensor
task db:start         # Start local dev postgres container
task db:stop          # Stop local dev postgres container
```

## Security Vulnerability Workflow

When fixing GitHub-reported vulnerabilities, read the GitHub Security Advisory or Dependabot alert first. Use the advisory's "patched versions" or explicit fix-version guidance as the target upgrade, then verify locally with the relevant audit command (`task audit:fe`, `task audit:be`, or the full `task audit`) before committing.

## Architecture Patterns

### Backend code health

Do not add optional plugin interfaces for required metadata. If every reader must provide it, put it in the main metadata struct.

Avoid long constructor signatures. Use a small input/deps struct once a constructor exceeds 4-5 parameters.

`internal/plugins.Registry` is a catalog, not an application assembler. Daemon/runtime wiring belongs outside the registry.

Define Go interfaces at consumer boundaries. Do not create package-local interfaces beside a single implementation unless a decorator or test boundary needs it.

Do not call instrumentation helpers from inside repository implementations. Wrap store/repository interfaces with decorators that own logging, metrics, and tracing, then delegate to the concrete implementation.

Store instrumentation decorators must keep the delegated call visible in each method: start the span inline, call `s.next.Method(...)` directly, record the operation result, and return. Do not hide store calls inside callback helpers such as `observe1`/`observe2`.

Keep concrete Postgres repositories focused on database behavior. New store behavior must live in the owning repository file, not in `internal/store/store.go`.

Do not add provider-specific helpers to `pkg/api`; Gmail/Thunderbird-specific behavior belongs in that reader package.

Do not use `context.Background()` inside request or daemon paths when a caller context is available.

Keep `slog` as the application logging API. Use OpenTelemetry for traces and metrics, not log export.

Do not add stdout trace or metric exporters. Stdout is for logs only; Expensor telemetry exporters are `none` or `otlp`.

Do not put high-cardinality or sensitive values in metrics or trace attributes, including email bodies, snippets, sender addresses, message IDs, transaction IDs, merchant names, and raw SQL.

### Plugin system
Reader and writer plugins implement interfaces in `pkg/api`. Plugins are registered in `cmd/server/main.go` and selected at runtime via the web UI (not env vars). Adding a new reader means implementing `plugins.ReaderPlugin` and registering it in the registry.

### Storer interface
`internal/api/store.go` defines `Storer` — a narrow interface over `*store.Store` used by HTTP handlers. It is **not** `*store.Store` itself. When adding a new store method that a handler needs, add it to `Storer` first, then implement it on `*store.Store`. The compile-time assertion `var _ Storer = (*store.Store)(nil)` at the bottom of `store.go` will catch mismatches.

### Handler tests
Unit tests in `internal/api/handlers_test.go` use `mockStore` (not a real DB). When adding a new `Storer` method, add the corresponding mock method too. Use `httptest.NewRequestWithContext(context.Background(), ...)` — not `httptest.NewRequest`.

### Integration tests
`internal/store/store_test.go` and `pkg/writer/postgres/postgres_test.go` spin up a real Postgres container via testcontainers-go. Skip with `-short`. These tests live in `package store_test` / `package postgres_test` (external test packages).

### Migrations
SQL files in `backend/migrations/` are embedded into the binary and run automatically on startup. Name new files `NNN_description.sql` (next sequential number). Migrations use `IF NOT EXISTS` and `ON CONFLICT DO NOTHING` — they must be idempotent.

### Rules and fixtures
Bundled extraction rules live in `content/rules.json` as a versioned v2 document. Treat this as the source of truth for rule edits and contributions. The repository currently also has `backend/cmd/server/content/rules.json` because the Go binary embeds files from that package path; keep that mirror in sync until `content/` is the only definitive rule location. Rules use exact sender matching with `sender_emails`; add every supported sender address explicitly. Rule source is structured as `source.type`, `source.label`, and `source.bank`. When bundled rules introduce a new type or bank, update the matching `presets.source_types` or `presets.banks` entry too.

Rule email tests live under `tests/data/rule-emails` as self-contained `.rule.fixture` files with YAML front matter plus the raw email body below the closing `---`. Use one email per file and name each file `<bank>_<source-type>_<case>.rule.fixture`, with lowercase slug segments such as `hdfc_credit-card_classic-spend.rule.fixture`. Fixtures must not include regexes or timestamps; the table-driven runner loads the named rule from `rules.json`, uses the fixture filename as the subtest name, and asserts sender/subject matching plus amount, merchant, and currency extraction.

### Screenshot Workflow
Use `task screenshots:live` for the real-page screenshots that are checked into `docs/screenshots/`. It captures both dashboard and transactions pages in light and dark themes using a high-resolution preset; keep the viewport and scale high enough that wide tables and charts do not get cramped. The README hero should point at `docs/screenshots/transactions-light.png`.

Keep screenshot assets and their instructions together in `docs/screenshots/README.md`. If the README image or screenshot directory contents change, update that file in the same change so future agents can reproduce the assets without reverse-engineering the capture flow.

### Recent Branch Lessons
Preserve v2 rule data end-to-end. If you touch rule parsing, seeding, or imports, make sure `sender_emails`, `source.type`, `source.label`, and `source.bank` survive the round-trip through API, store, migrations, and seeded data. Component tests that exercise rules or transactions often need their fixtures updated in lockstep.

When changing seeded demo data for screenshots, prefer realistic merchant names and balanced distributions across categories, buckets, labels, banks, and source types. The goal is to make the dashboard and transactions screenshots visually representative, not minimally populated.

Do not create ad hoc screenshot commands in shell history. If capture settings matter, encode them in `Taskfile.yml` or a script under `frontend/scripts/` so they can be reused.

### Configuration
All env config flows through `pkg/config/config.go` using koanf. Only four env prefixes are loaded: `EXPENSOR_`, `GMAIL_`, `THUNDERBIRD_`, `POSTGRES_`. Do not add config fields under other prefixes.

### Internationalization
Frontend i18n lives under `frontend/src/i18n/`. English is the source catalog in `messages.ts`; add new locales by copying the full English key set and translating values without renaming keys. Components should use `useI18n()` and `MessageKey` for shared navigation, command, settings tab, repeated control label, and other extracted strings. User-facing currency, dates, month labels, and counts should use helpers from `frontend/src/i18n/format.ts`. See `docs/i18n/adding-translations.md` and `docs/i18n/string-extraction.md`.

New frontend changes must be i18n-friendly by default. When touching existing frontend code, move newly affected user-facing strings into the i18n catalog instead of adding more hardcoded copy.

## Frontend Design Language Rules

**Never use browser-native UI controls.** This project has a custom dark-themed design language. The following must never appear in frontend code:

- `<select>` — use `InlineSelect` or a custom styled dropdown
- `<datalist>` — use a custom combobox component (see `SourceCombobox` in `RuleForm.tsx` as a reference pattern)
- `confirm()` / `alert()` / `prompt()` — use `ConfirmModal` from `@/components/ConfirmModal`
- Native `title` attribute for tooltips — use CSS `group-hover` or `position:fixed` + mouse event state (see `Rules.tsx`)

**Overflow clipping — tooltips and dropdowns (CRITICAL):** Any absolutely-positioned UI (tooltips, dropdowns, popovers) inside a container with `overflow-hidden`, `overflow-x-auto`, or `overflow-y-auto` WILL be clipped. This has caused multiple bugs. The rule without exception: use `position: fixed` + `getBoundingClientRect()` + `createPortal(…, document.body)` to escape the clipping ancestor. Never use `position: absolute` for floating UI inside sidebar, table rows, or any scrollable container. See `Sidebar.tsx` (toggle tooltip), `LabelCombobox.tsx`, and `InlineSelect.tsx` for the established pattern.

**Dropdown surfaces:** Dropdowns, combobox menus, and listboxes must use an opaque semantic surface such as `bg-card text-card-foreground border border-border shadow-lg`. Do not use background utility classes that are not defined in `frontend/tailwind.config.js`; they compile away and create transparent menus.

**Disabled elements and mouse events:** Disabled form elements (`<button disabled>`, `<input disabled>`) do not fire `mouseenter`/`mouseleave` in browsers. Wrap them in a `<span>` to handle hover events when needed.

**URL state persistence (required):** Any page with tabs, filters, or navigation state MUST persist that state in the URL via `useSearchParams` (react-router-dom) so that refreshing or duplicating the tab restores the same view. Do NOT use `useState` for tab selection or filter state that the user might want to share or return to. The `Transactions` page is the reference implementation. Pages not yet compliant (Settings tabs, Dashboard heatmap year) should be migrated when touched.

**Slide-in notifications:** Use `SlideNotification` from `@/components/SlideNotification` for any transient action prompts that offer 2 choices and auto-dismiss after a timeout. Do not use inline absolute-positioned prompts inside table cells — they are clipped by overflow containers.

See `frontend/README.md` for the full color system, component patterns, and spacing rules.

## Lint Rules to Know

The prod linter is strict. Common traps:

- **noctx**: use `httptest.NewRequestWithContext` in tests, never `httptest.NewRequest`
- **lll**: max line length is 160 characters
- **misspell**: use American spellings (`canceled`, `labeled`)
- **revive argument-limit**: max 5 params per function — use `//nolint:revive` with a justification for DI constructors and test helpers
- **gosec G703/G705**: file path taint analysis — add `//nolint:gosec // path built from validated reader name` when paths are safe but taint-analysis can't prove it
- **gocognit**: max cognitive complexity 20 — extract helper functions if a function grows complex
- **import-shadowing**: don't name local variables the same as imported packages (e.g. don't shadow `url`, `path`, `time`)

Run `task lint:be:prod` before every commit. It must report `0 issues`.

## Git Conventions

- Branch format: `pr/<short-description>` (no Jira ticket for this repo)
- Commits: imperative mood, Tim Pope style, `--no-gpg-sign`
- Never commit to `main` directly — branch protection requires PRs (bypass only for docs/chore)
- Check `.pre-commit-config.yaml` before committing if it exists
- Always use the repository PR template at `.github/PULL_REQUEST_TEMPLATE.md` when creating or updating PR descriptions. Do not compose PR bodies from scratch or omit template sections.

## Testing Strategy

Follow TDD for backend and frontend work:
1. Write failing tests first
2. Confirm they fail
3. Implement to make them pass
4. Run full suite (`task test`) to catch regressions

No new feature ships without tests. When enhancing an existing feature, first inspect the current test coverage for that behavior. If no appropriate tests exist, add the requisite tests and confirm they fail before changing production code.

Always leverage the existing test infrastructure instead of inventing ad hoc checks:

- Backend unit tests cover handlers, services, extractors, and store-facing behavior.
- Backend component tests live under `tests/component` and run through `task test:be:component`.
- Backend contract tests live under `tests/contract` and run through `task test:be:contract`.
- Frontend unit and component tests use Vitest and run through `task test:fe`.
- Frontend mocked Playwright E2E tests live under `frontend/playwright` and run through `task test:fe:e2e`.
- Full-stack Playwright smoke tests run through `task test:fe:e2e:smoke`.

Choose the narrowest test that proves the behavior, then run the broader relevant suite before finishing. For UI behavior, prefer a component test for isolated state/rendering logic and a Playwright test for user-visible flows, routing, persistence, or backend integration.

Unit tests mock the store via `mockStore`. Integration tests hit a real container — run them explicitly when changing `store/` or `writer/postgres/`.
