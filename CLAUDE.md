# Expensor — Agent Instructions

This file is loaded by Codex through `AGENTS.md` and by Claude Code. Keep it focused on guidance that prevents likely agent mistakes. Prefer discovering layout, package names, and routine commands from the repository instead of duplicating them here.

## Command Policy

Always prefer `task` over bare `go`/`npm` commands. Task targets handle working directories, env loading, and project-specific tooling.

Use `task --list-all` to discover available targets. Common defaults:

```bash
task dev              # Start the local app stack
task fmt              # Format backend and frontend
task test             # Default backend + frontend tests
task test:be          # Backend tests; some tests use Docker
task test:fe          # Frontend unit/component tests
task lint:be:prod     # Strict backend lint; must report 0 issues before commit
task openapi:check    # Regenerate OpenAPI and fail on committed artifact drift
```

## Security Vulnerability Workflow

When fixing GitHub-reported vulnerabilities, read the GitHub Security Advisory or Dependabot alert first. Use the advisory's "patched versions" or explicit fix-version guidance as the target upgrade, then verify locally with the relevant audit command (`task audit:fe`, `task audit:be`, or the full `task audit`) before committing.

## Architecture Patterns

### Backend code health

Production Go code must use `backend/pkg/errors`. Outside `*_test.go` files and `backend/pkg/errors` itself, do not import the standard-library `errors` package or call `fmt.Errorf`. Use `errors.E` with a stable operation name and useful kind when wrapping failures at package/application boundaries; leaf validation errors may use a kind and message without an operation when the exact message is part of the existing contract. Use the project package's `Is`, `As`, `Join`, and `Unwrap` helpers for error-chain operations. Golangci-lint enforces these restrictions.

Do not add optional plugin interfaces for required metadata. If every reader must provide it, put it in the main metadata struct.

Use Go generics when they improve compile-time type safety or remove meaningful repeated algorithms across concrete types. Do not introduce generics solely to replace `any` when the underlying library still relies on reflection or when call sites do not become clearer. Do not claim a generics performance benefit without measuring the relevant path. For tag-driven decoders and validators, cache unavoidable local reflection metadata by concrete DTO type when it is reused.

Prefer standard-library constants and helpers over manually derived equivalents. For example, use `math.MaxInt` instead of calculating the platform maximum integer with bit operations.

Prefer carrying a dependency's native type through configuration and internal APIs when it represents the same domain value. Convert only at genuine representation or unit boundaries; avoid repeated narrowing casts and lint suppressions.

Avoid long constructor signatures. Use a small input/deps struct once a constructor exceeds 4-5 parameters.

`internal/plugins.Registry` is a catalog, not an application assembler. Daemon/runtime wiring belongs outside the registry.

Define Go interfaces at consumer boundaries. Do not create package-local interfaces beside a single implementation unless a decorator or test boundary needs it.

Do not call instrumentation helpers from inside repository implementations. Wrap store/repository interfaces with decorators that own logging, metrics, and tracing, then delegate to the concrete implementation.

Store instrumentation decorators must keep the delegated call visible in each method: start the span inline, call the relevant behavior-boundary dependency directly, record the operation result, and return. Do not hide store calls inside callback helpers such as `observe1`/`observe2`.

Keep concrete Postgres repositories focused on database behavior. New store behavior must live in the owning repository file, not in `internal/store/store.go`.

Do not add provider-specific helpers to `pkg/api`; Gmail/Thunderbird-specific behavior belongs in that reader package.

Do not use `context.Background()` inside request or daemon paths when a caller context is available.

Keep `slog` as the application logging API. Use OpenTelemetry for traces and metrics, not log export.

Do not add stdout trace or metric exporters. Stdout is for logs only; Expensor telemetry exporters are `none` or `otlp`.

Do not put high-cardinality or sensitive values in metrics, trace attributes, span events, or span status descriptions, including email bodies, snippets, sender addresses, message IDs, transaction IDs, merchant names, raw error strings, and raw SQL.

### HTTP API design

Model resources and collections before adding routes. Prefer standard HTTP methods over action-oriented endpoint suffixes:

- Use collection query parameters for search, filtering, sorting, and pagination. Do not add a parallel `/search` route for the same collection.
- Use `POST` to create collection members, `PUT` to replace a resource or idempotently create one at a known URI, `PATCH` for partial updates, and `DELETE` for removal.
- Model related state as subresources. Do not put identity-bearing fields in a `DELETE` request body; put the identity in the path or query string.
- Model application settings as singleton resources with `GET` and `PATCH` or `PUT`, rather than separate getter/setter routes for each field.
- Do not introduce stable IDs unless the resource has a real independent identity or lifecycle need. URL-encoded natural identifiers are acceptable while router and proxy behavior remains adequate.
- Keep genuinely distinct read models such as facets, dashboards, and charts separate. Commands and protocol transitions such as OAuth, daemon control, import, and export may remain action-oriented when resource semantics would be artificial.
- When consolidating internal routes, update handlers, frontend clients, mocks, OpenAPI, contract allowlists, and documentation together. Do not retain compatibility aliases unless compatibility is an explicit requirement.

Decode and validate request DTOs at the HTTP boundary, close to the handler that owns their semantics. Do not use generic middleware for request-specific DTO validation or register every DTO globally. Return `400 Bad Request` for malformed syntax and `422 Unprocessable Entity` for well-formed requests that fail semantic validation.

Validation errors must use the structured `field`, `location`, and `message` detail schema. Do not expose validator tag names or other implementation details as error codes. Validate the complete request before persistence so multi-field updates cannot partially apply.

HTTP validation improves contracts and error reporting; it is not SQL-injection protection. Continue using parameterized SQL at the repository boundary, and retain domain/store invariants needed by non-HTTP callers or required for data integrity.

### Plugin system
Reader plugins implement interfaces in `pkg/api`. Built-in plugins are registered in `internal/app/readers.go` and selected at runtime via the web UI (not env vars). Adding a new reader means implementing the plugin capabilities and registering the provider in application composition.

### Storer interface
`internal/httpapi/store.go` defines `Storer` — a narrow interface over the persistence surface used by HTTP handlers. It is not the concrete Postgres store. When adding a new store method that a handler needs, add it to `Storer` first, then implement it on the concrete backend store and the instrumented wrapper. The compile-time assertions in `store.go` catch mismatches.

### Handler tests
Unit tests in `internal/api/handlers_test.go` use `mockStore` (not a real DB). When adding a new `Storer` method, add the corresponding mock method too. Use `httptest.NewRequestWithContext(context.Background(), ...)` — not `httptest.NewRequest`.

### Integration tests
`internal/store/postgres/store_test.go` and `internal/store/postgres/ingestion_test.go` spin up a real Postgres container via testcontainers-go. Skip with `-short`. These tests live in `package postgres` so test-only helpers can access unexported internals without adding production methods.

### Migrations
Postgres SQL files in `backend/internal/store/postgres/migrations/` are embedded into the binary and run automatically on startup. Name new files `NNN_description.sql` (next sequential number). Migrations use `IF NOT EXISTS` and `ON CONFLICT DO NOTHING` — they must be idempotent.

### Rules and fixtures
Bundled extraction rules live in `backend/internal/catalog/content/rules.json` as a versioned v2 document. Treat this as the source of truth for rule edits and contributions. Rules use exact sender matching with `sender_emails`; add every supported sender address explicitly. Rule source is structured as `source.type`, `source.label`, and `source.bank`. When bundled rules introduce a new type or bank, update the matching `presets.source_types` or `presets.banks` entry too.

Rule email tests live under `tests/data/rule-emails` as self-contained `.rule.fixture` files with YAML front matter plus the raw email body below the closing `---`. Use one email per file and name each file `<bank>_<source-type>_<case>.rule.fixture`, with lowercase slug segments such as `hdfc_credit-card_classic-spend.rule.fixture`. Fixtures must not include regexes or timestamps; the table-driven runner loads the named rule from `rules.json`, uses the fixture filename as the subtest name, and asserts sender/subject matching plus amount, merchant, and currency extraction.

### Screenshot Workflow
Use `task screenshots:live` for the real-page screenshots that are checked into `docs/screenshots/`. It captures both dashboard and transactions pages in light and dark themes using a high-resolution preset; keep the viewport and scale high enough that wide tables and charts do not get cramped. The README hero should point at `docs/screenshots/transactions-light.png`.

Keep screenshot assets and their instructions together in `docs/screenshots/README.md`. If the README image or screenshot directory contents change, update that file in the same change so future agents can reproduce the assets without reverse-engineering the capture flow.

### Recent Branch Lessons
Preserve v2 rule data end-to-end. If you touch rule parsing, seeding, or imports, make sure `sender_emails`, `source.type`, `source.label`, and `source.bank` survive the round-trip through API, store, migrations, and seeded data. Component tests that exercise rules or transactions often need their fixtures updated in lockstep.

When changing seeded demo data for screenshots, prefer realistic merchant names and balanced distributions across categories, buckets, labels, banks, and source types. The goal is to make the dashboard and transactions screenshots visually representative, not minimally populated.

Do not create ad hoc screenshot commands in shell history. If capture settings matter, encode them in `Taskfile.yml` or a script under `frontend/scripts/` so they can be reused.

### Configuration
All application environment config flows through `pkg/config/config.go` using `kelseyhightower/envconfig`. Do not read application environment variables directly from feature packages. OS discovery variables such as Windows `APPDATA` may remain local to the platform-specific code that consumes them.

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

**Document titles:** Route-level document titles must match the visible page state. Plain routes use the page title only (`Transactions`); tabbed routes use `Page - Active Tab` (`Settings - Account`, `Expense Groups - Categories`). Keep `frontend/src/lib/documentTitle.ts` and its tests updated when adding tabs or tabbed pages.

**Copy buttons:** Copy actions should use an icon-only button with an accessible label. When the action is not self-evident or appears in a dense surface, show a fixed/portal tooltip with the action label on hover/focus, and change the tooltip to `Copied!` only after clipboard write succeeds. Do not show raw copied values inline unless the value is meant to be inspected, such as a one-time token reveal.

**Slide-in notifications:** Use `SlideNotification` from `@/components/SlideNotification` for any transient action prompts that offer 2 choices and auto-dismiss after a timeout. Do not use inline absolute-positioned prompts inside table cells — they are clipped by overflow containers.

**Combobox/listbox reuse:** Do not implement custom combobox, dropdown, autocomplete, or listbox mechanics from scratch. Use `frontend/src/components/Combobox.tsx` for fixed body-portal rendering, dropdown positioning, outside-click closing, Escape/ArrowUp/ArrowDown/Enter handling, highlighted option state, and ARIA combobox/listbox/option wiring. Page components may own filtering, labels, API calls, create-option behavior, and domain-specific rendering. Before adding a new dropdown-like control, grep for the shared combobox primitive and extend it only when the behavior is broadly reusable; document any one-off exception in the PR.

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

- Branch format: `<type>/<short-description>`, using one of `feat`, `fix`, `docs`, `style`, `refactor`, `test`, or `chore`. Use lowercase, hyphen-separated descriptions and no Jira ticket.
- Commit and PR-title format: `<type>(<optional-scope>): <subject>`. Use a supported type, an optional concise lowercase scope, and a present-tense subject. For example: `refactor(daemon): centralize scan configuration`.
- Commits use `--no-gpg-sign`.
- Never commit to `main` directly — branch protection requires PRs (bypass only for docs/chore)
- Check `.pre-commit-config.yaml` before committing if it exists
- Always use the repository PR template at `.github/PULL_REQUEST_TEMPLATE.md` when creating or updating PR descriptions. Do not compose PR bodies from scratch or omit template sections.

## Testing Strategy

Follow TDD for backend and frontend work:
1. Write failing tests first
2. Confirm they fail
3. Implement to make them pass
4. Run the narrow relevant suite, then `task test`; add component, contract, or browser suites when the touched area requires them

No new feature ships without tests. When enhancing an existing feature, first inspect the current test coverage for that behavior. If no appropriate tests exist, add the requisite tests and confirm they fail before changing production code.

Always leverage the existing test infrastructure instead of inventing ad hoc checks:

- Backend unit tests cover handlers, services, extractors, and store-facing behavior.
- Backend component tests live under `tests/component` and run through `task test:be:component`.
- Backend contract tests live under `tests/contract` and run through `task test:be:contract`.
- Frontend unit and component tests use Vitest and run through `task test:fe`.
- Frontend mocked Playwright E2E tests live under `frontend/playwright` and run through `task test:fe:e2e`.
- Full-stack Playwright smoke tests run through `task test:fe:e2e:smoke`.

Choose the narrowest test that proves the behavior, then run the broader relevant suite before finishing. For UI behavior, prefer a component test for isolated state/rendering logic and a Playwright test for user-visible flows, routing, persistence, or backend integration.

Do not create tests just to satisfy TDD for mechanical refactors, dead-code removal, interface cleanup, constructor signature cleanup, or other changes whose correctness is already proven by compilation, linting, and existing behavior tests. In those cases, use the existing suite as the safety net. Add a new test only when the refactor exposes or preserves a meaningful public behavior, contract, parsing rule, persistence rule, routing behavior, or accessible user flow that is not already covered.

Unit tests mock the store via `mockStore`. Integration tests hit a real container — run them explicitly when changing `store/` or ingestion behavior.
